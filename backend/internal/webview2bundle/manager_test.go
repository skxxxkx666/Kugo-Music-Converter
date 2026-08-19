package webview2bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureRuntimeExtractsAndReusesPayload(t *testing.T) {
	originalExpand := expandRuntimeCAB
	originalGrant := grantRuntimeAccess
	t.Cleanup(func() {
		expandRuntimeCAB = originalExpand
		grantRuntimeAccess = originalGrant
	})

	expandCalls := 0
	grantCalls := 0
	expandRuntimeCAB = func(_ string, destination string) error {
		expandCalls++
		browserDir := filepath.Join(destination, "FixedRuntime")
		if err := os.MkdirAll(browserDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(browserDir, "msedgewebview2.exe"), []byte("runtime"), 0o755)
	}
	grantRuntimeAccess = func(string) error {
		grantCalls++
		return nil
	}

	payload := []byte("cab payload")
	info := PayloadInfo{CAB: payload, Version: "151.0.4129.93", ExpectedSHA256: sha256Hex(payload)}
	cacheRoot := t.TempDir()

	first, err := EnsureRuntime(cacheRoot, info)
	if err != nil {
		t.Fatalf("EnsureRuntime() first call error = %v", err)
	}
	if !first.Extracted {
		t.Fatal("EnsureRuntime() first call did not report extraction")
	}
	if filepath.Base(first.BrowserPath) != "FixedRuntime" {
		t.Fatalf("EnsureRuntime() BrowserPath = %q", first.BrowserPath)
	}

	second, err := EnsureRuntime(cacheRoot, info)
	if err != nil {
		t.Fatalf("EnsureRuntime() second call error = %v", err)
	}
	if second.Extracted {
		t.Fatal("EnsureRuntime() extracted an already valid cached runtime")
	}
	if second.BrowserPath != first.BrowserPath {
		t.Fatalf("EnsureRuntime() reused path = %q, want %q", second.BrowserPath, first.BrowserPath)
	}
	if expandCalls != 1 {
		t.Fatalf("expand calls = %d, want 1", expandCalls)
	}
	if grantCalls != 1 {
		t.Fatalf("grant calls = %d, want 1", grantCalls)
	}
}

func TestEnsureRuntimeRejectsChecksumMismatch(t *testing.T) {
	payload := []byte("cab payload")
	_, err := EnsureRuntime(t.TempDir(), PayloadInfo{
		CAB:            payload,
		Version:        "151.0.4129.93",
		ExpectedSHA256: sha256Hex([]byte("different")),
	})
	if err == nil {
		t.Fatal("EnsureRuntime() error = nil, want checksum mismatch")
	}
}

func TestEnsureRuntimeRequiresPayload(t *testing.T) {
	_, err := EnsureRuntime(t.TempDir(), PayloadInfo{})
	if !errors.Is(err, ErrPayloadUnavailable) {
		t.Fatalf("EnsureRuntime() error = %v, want ErrPayloadUnavailable", err)
	}
}

func TestEnsureRuntimeRejectsInvalidVersion(t *testing.T) {
	payload := []byte("cab payload")
	_, err := EnsureRuntime(t.TempDir(), PayloadInfo{
		CAB:            payload,
		Version:        "latest",
		ExpectedSHA256: sha256Hex(payload),
	})
	if err == nil {
		t.Fatal("EnsureRuntime() error = nil, want invalid version")
	}
}

func sha256Hex(value []byte) string {
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}
