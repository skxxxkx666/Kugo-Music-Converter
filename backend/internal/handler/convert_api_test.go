package handler

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

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
