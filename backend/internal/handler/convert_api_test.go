package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"kugo-music-converter/internal/config"
)

func TestHandleConfigReportsOneGiBDefaultFileLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	h := &ConvertHandler{cfg: cfg, baseDir: t.TempDir()}
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()

	h.HandleConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var response struct {
		Limits struct {
			MaxFileSizeMB    int `json:"maxFileSizeMB"`
			MaxUploadTotalMB int `json:"maxUploadTotalMB"`
		} `json:"limits"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	if response.Limits.MaxFileSizeMB != 1024 {
		t.Fatalf("expected API file limit 1024 MiB, got %d", response.Limits.MaxFileSizeMB)
	}
	if response.Limits.MaxUploadTotalMB != 2028 {
		t.Fatalf("expected API upload total limit 2028 MiB, got %d", response.Limits.MaxUploadTotalMB)
	}
}

func TestRequestErrorStatusUsesPayloadTooLarge(t *testing.T) {
	if got := requestErrorStatus(NewAppError(ErrFileTooLarge, "too large", nil)); got != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, got)
	}
	if got := requestErrorStatus(errors.New("bad request")); got != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, got)
	}
}

func TestNormalizeLocalInputPath(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "song.kgg")
	got, err := normalizeLocalInputPath(target)
	if err != nil {
		t.Fatalf("normalizeLocalInputPath returned error: %v", err)
	}
	if got == "" || !filepath.IsAbs(got) {
		t.Fatalf("expected absolute path, got %q", got)
	}

	if _, err := normalizeLocalInputPath(""); err == nil {
		t.Fatalf("expected empty path to fail")
	}

	if runtime.GOOS == "windows" {
		if _, err := normalizeLocalInputPath(`\\server\share\song.kgg`); err == nil {
			t.Fatalf("expected UNC path to fail on windows")
		}
	}
}

func TestParseInputPathItemsLocalAbsolute(t *testing.T) {
	root := t.TempDir()
	allowedFile := filepath.Join(root, "ok.kgg")
	if err := os.WriteFile(allowedFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	rawAllowed, _ := json.Marshal([]string{allowedFile})
	items, err := parseInputPathItems(string(rawAllowed))
	if err != nil {
		t.Fatalf("parseInputPathItems returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Path != filepath.Clean(allowedFile) {
		t.Fatalf("unexpected item path: %s", items[0].Path)
	}

	rawRelative, _ := json.Marshal([]string{"relative.kgg"})
	_, err = parseInputPathItems(string(rawRelative))
	if err == nil {
		t.Fatalf("expected relative path denied error")
	}

	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != ErrInputPathDenied {
		t.Fatalf("expected code %s, got %s", ErrInputPathDenied, appErr.Code)
	}
}
