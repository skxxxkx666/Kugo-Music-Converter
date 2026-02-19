package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"kugo-music-converter/internal/logger"
)

const sseWriteTimeout = 10 * time.Second

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

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	closed := false
	done := r.Context().Done()
	stopFn := func() bool {
		select {
		case <-done:
			closed = true
			return true
		default:
			return false
		}
	}

	onEvent := func(name string, payload any) {
		if closed {
			return
		}
		if err := writeSSEEvent(w, name, payload); err != nil {
			closed = true
		}
	}

	summary := h.executeBatch(r.Context(), req, stopFn, onEvent)
	onEvent("complete", map[string]any{
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
