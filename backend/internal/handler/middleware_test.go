package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsTrustedLocalURL(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		allow bool
	}{
		{name: "localhost", raw: "http://localhost:8080", allow: true},
		{name: "loopback-v4", raw: "http://127.0.0.1:3000/path", allow: true},
		{name: "loopback-v6", raw: "http://[::1]:8080", allow: true},
		{name: "null-origin", raw: "null", allow: false},
		{name: "remote", raw: "https://evil.example.com", allow: false},
		{name: "invalid", raw: "://broken", allow: false},
	}

	for _, tc := range cases {
		got := isTrustedLocalURL(tc.raw)
		if got != tc.allow {
			t.Fatalf("%s: expected %v, got %v", tc.name, tc.allow, got)
		}
	}
}

func TestWithLocalOriginGuard(t *testing.T) {
	target := withLocalOriginGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name       string
		path       string
		origin     string
		referer    string
		wantStatus int
	}{
		{name: "api-allowed-origin", path: "/api/config", origin: "http://localhost:8080", wantStatus: http.StatusOK},
		{name: "api-allowed-no-headers", path: "/api/config", wantStatus: http.StatusOK},
		{name: "api-blocked-origin", path: "/api/config", origin: "https://evil.example.com", wantStatus: http.StatusForbidden},
		{name: "api-blocked-referer", path: "/api/config", referer: "https://evil.example.com/page", wantStatus: http.StatusForbidden},
		{name: "non-api-skip-check", path: "/", origin: "https://evil.example.com", wantStatus: http.StatusOK},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		if tc.referer != "" {
			req.Header.Set("Referer", tc.referer)
		}
		rec := httptest.NewRecorder()
		target.ServeHTTP(rec, req)
		if rec.Code != tc.wantStatus {
			t.Fatalf("%s: expected %d, got %d", tc.name, tc.wantStatus, rec.Code)
		}
	}
}
