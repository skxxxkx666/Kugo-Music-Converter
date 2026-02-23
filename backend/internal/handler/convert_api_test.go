package handler

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPathWithinWhitelist(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "a", "b", "song.kgg")
	outside := filepath.Join(t.TempDir(), "song.kgg")

	if !pathWithinWhitelist(inside, []string{root}) {
		t.Fatalf("expected inside path to pass whitelist check")
	}
	if pathWithinWhitelist(outside, []string{root}) {
		t.Fatalf("expected outside path to be rejected")
	}
}

func TestParseInputPathItemsWhitelist(t *testing.T) {
	allowedRoot := t.TempDir()
	allowedFile := filepath.Join(allowedRoot, "ok.kgg")
	if err := os.WriteFile(allowedFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write allowed file: %v", err)
	}

	rawAllowed, _ := json.Marshal([]string{allowedFile})
	items, err := parseInputPathItems(string(rawAllowed), []string{allowedRoot})
	if err != nil {
		t.Fatalf("parseInputPathItems allowed path error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	deniedFile := filepath.Join(t.TempDir(), "deny.kgg")
	if err := os.WriteFile(deniedFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write denied file: %v", err)
	}
	rawDenied, _ := json.Marshal([]string{deniedFile})
	_, err = parseInputPathItems(string(rawDenied), []string{allowedRoot})
	if err == nil {
		t.Fatalf("expected whitelist denied error")
	}

	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != ErrInputPathDenied {
		t.Fatalf("expected code %s, got %s", ErrInputPathDenied, appErr.Code)
	}
}
