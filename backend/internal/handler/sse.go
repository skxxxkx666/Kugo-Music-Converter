package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"kugo-music-converter/internal/logger"
)

const sseWriteTimeout = 10 * time.Second
const sseProgressThrottleInterval = 100 * time.Millisecond

type sseProgressThrottler struct {
	mu       sync.Mutex
	interval time.Duration
	send     func(any)
	pending  any
	timer    *time.Timer
	stopped  bool
}

func newSSEProgressThrottler(interval time.Duration, send func(any)) *sseProgressThrottler {
	if interval <= 0 {
		interval = sseProgressThrottleInterval
	}
	return &sseProgressThrottler{
		interval: interval,
		send:     send,
	}
}

func (t *sseProgressThrottler) Submit(payload any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped {
		return
	}
	t.pending = payload
	if t.timer != nil {
		return
	}
	t.timer = time.AfterFunc(t.interval, t.flushFromTimer)
}

func (t *sseProgressThrottler) flushFromTimer() {
	payload := t.popPending()
	if payload == nil || t.send == nil {
		return
	}
	t.send(payload)
}

func (t *sseProgressThrottler) popPending() any {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.timer != nil {
		t.timer = nil
	}
	if t.stopped {
		t.pending = nil
		return nil
	}
	payload := t.pending
	t.pending = nil
	return payload
}

func (t *sseProgressThrottler) Flush() {
	t.mu.Lock()
	if t.timer != nil {
		_ = t.timer.Stop()
		t.timer = nil
	}
	if t.stopped {
		t.pending = nil
		t.mu.Unlock()
		return
	}
	payload := t.pending
	t.pending = nil
	t.mu.Unlock()

	if payload != nil && t.send != nil {
		t.send(payload)
	}
}

func (t *sseProgressThrottler) Stop() {
	t.mu.Lock()
	if t.timer != nil {
		_ = t.timer.Stop()
		t.timer = nil
	}
	t.pending = nil
	t.stopped = true
	t.mu.Unlock()
}

func writeSSEEvent(w http.ResponseWriter, event string, payload any) error {
	ctrl := http.NewResponseController(w)
	if err := ctrl.SetWriteDeadline(time.Now().Add(sseWriteTimeout)); err != nil {
		logger.Debugf("set SSE write deadline failed: %v", err)
	}
	defer func() {
		if err := ctrl.SetWriteDeadline(time.Time{}); err != nil {
			logger.Debugf("reset SSE write deadline failed: %v", err)
		}
	}()

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func (h *ConvertHandler) HandleConvertStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	if missing := h.runtimeMissingTools(); len(missing) > 0 {
		writeError(w, http.StatusServiceUnavailable, NewAppError(ErrRuntimeMissing, strings.Join(missing, ","), nil))
		return
	}

	req, err := h.parseConvertRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer req.Cleanup()

	if h.isShuttingDown() {
		writeError(w, http.StatusServiceUnavailable, NewAppError(ErrRuntimeMissing, "服务正在关闭，请稍后重试", nil))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	closed := atomic.Bool{}
	var sendMu sync.Mutex
	done := r.Context().Done()
	stopFn := func() bool {
		select {
		case <-done:
			closed.Store(true)
			return true
		default:
			return false
		}
	}

	var sendNow func(name string, payload any)
	var progressThrottler *sseProgressThrottler

	onEvent := func(name string, payload any) {
		if name == "progress" {
			progressThrottler.Submit(payload)
			return
		}
		progressThrottler.Flush()
		sendNow(name, payload)
	}

	sendNow = func(name string, payload any) {
		sendMu.Lock()
		defer sendMu.Unlock()
		if closed.Load() {
			return
		}
		if err := writeSSEEvent(w, name, payload); err != nil {
			closed.Store(true)
		}
	}
	progressThrottler = newSSEProgressThrottler(sseProgressThrottleInterval, func(payload any) {
		sendNow("progress", payload)
	})
	defer progressThrottler.Stop()

	summary := h.executeBatch(r.Context(), req, stopFn, onEvent)
	h.registerPreviewFiles(summary.Results)
	progressThrottler.Flush()
	sendNow("complete", map[string]any{
		"success":      summary.Success,
		"failed":       summary.Failed,
		"total":        summary.Total,
		"outputDir":    summary.OutputDir,
		"durationMs":   summary.DurationMs,
		"cancelled":    summary.Cancelled,
		"outputFormat": summary.OutputFormat,
		"mp3Quality":   summary.MP3Quality,
		"results":      summary.Results,
	})
}
