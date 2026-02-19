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
