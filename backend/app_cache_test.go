package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeCacheKeepsActiveRuntimeAndClearsOnlyManagedEntries(t *testing.T) {
	root := t.TempDir()
	activeRuntime := filepath.Join(root, "runtime", "active")
	staleRuntime := filepath.Join(root, "runtime", "stale")
	staleWebView := filepath.Join(root, "webview2", "old")
	updateFile := filepath.Join(root, "updates", "update.exe")
	outside := filepath.Join(t.TempDir(), "music.ncm")
	for path, content := range map[string]string{
		filepath.Join(activeRuntime, "ffmpeg.exe"): "active",
		filepath.Join(staleRuntime, "ffmpeg.exe"):  "stale-runtime",
		filepath.Join(staleWebView, "browser.exe"): "stale-webview",
		updateFile: "update",
		outside:    "music",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
	}

	info, entries, err := inspectRuntimeCache(root, []string{activeRuntime, outside})
	if err != nil {
		t.Fatalf("inspectRuntimeCache() error = %v", err)
	}
	if info.ReclaimableItems != 3 || info.RetainedBytes != int64(len("active")) {
		t.Fatalf("cache info = %#v", info)
	}
	result := clearRuntimeCacheEntries(entries)
	if result.RemovedItems != 3 || result.Warning != "" {
		t.Fatalf("clear result = %#v", result)
	}
	for _, path := range []string{filepath.Join(activeRuntime, "ffmpeg.exe"), outside} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("protected path %q was removed: %v", path, err)
		}
	}
	for _, path := range []string{staleRuntime, staleWebView, updateFile} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("stale cache %q still exists: %v", path, err)
		}
	}
}

func TestPathInsideRejectsSiblingPrefix(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	if pathInside(root, root+"-other") {
		t.Fatal("pathInside accepted sibling with shared prefix")
	}
}
