package handler

import (
	"net"
	"net/http"
	"strings"
	"time"

	"kugo-music-converter/internal/logger"
)

type statusResponseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

type writeTimeoutResponseWriter struct {
	http.ResponseWriter
	timeout time.Duration
}

func (w *writeTimeoutResponseWriter) setWriteDeadline() {
	if w.timeout <= 0 {
		return
	}
	ctrl := http.NewResponseController(w.ResponseWriter)
	if err := ctrl.SetWriteDeadline(time.Now().Add(w.timeout)); err != nil {
		logger.Debugf("set write deadline failed: %v", err)
	}
}

func (w *writeTimeoutResponseWriter) WriteHeader(code int) {
	w.setWriteDeadline()
	w.ResponseWriter.WriteHeader(code)
}

func (w *writeTimeoutResponseWriter) Write(p []byte) (int, error) {
	w.setWriteDeadline()
	return w.ResponseWriter.Write(p)
}

func (w *writeTimeoutResponseWriter) Flush() {
	w.setWriteDeadline()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *writeTimeoutResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.size += n
	return n, err
}

func (w *statusResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusResponseWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		if rw.status == 0 {
			rw.status = http.StatusOK
		}

		clientIP := getClientIP(r)
		logger.Debugf(
			"REQ %s %s status=%d bytes=%d ip=%s ua=%q took=%s",
			r.Method,
			r.URL.Path,
			rw.status,
			rw.size,
			clientIP,
			r.UserAgent(),
			time.Since(start),
		)
	})
}

func withWriteTimeout(next http.Handler, timeout time.Duration, skipFn func(*http.Request) bool) http.Handler {
	if timeout <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if skipFn != nil && skipFn(r) {
			next.ServeHTTP(w, r)
			return
		}
		writer := &writeTimeoutResponseWriter{
			ResponseWriter: w,
			timeout:        timeout,
		}
		defer func() {
			ctrl := http.NewResponseController(writer.ResponseWriter)
			if err := ctrl.SetWriteDeadline(time.Time{}); err != nil {
				logger.Debugf("reset write deadline failed: %v", err)
			}
		}()
		next.ServeHTTP(writer, r)
	})
}

func isSSERequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	return r.URL.Path == "/api/convert-stream"
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xr := r.Header.Get("X-Real-Ip"); xr != "" {
		return strings.TrimSpace(xr)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
