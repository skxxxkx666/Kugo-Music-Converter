package main

import (
	"strings"
	"testing"
)

func TestDesktopSupportedExtensionListMatchesFilterMap(t *testing.T) {
	if len(supportedDesktopExtList) != len(supportedDesktopExts) {
		t.Fatalf("extension list has %d items, map has %d", len(supportedDesktopExtList), len(supportedDesktopExts))
	}
	patterns := make([]string, 0, len(supportedDesktopExtList))
	for _, ext := range supportedDesktopExtList {
		if _, ok := supportedDesktopExts[ext]; !ok {
			t.Errorf("extension %q is listed but missing from filter map", ext)
		}
		patterns = append(patterns, "*"+ext)
	}
	if want := strings.Join(patterns, ";"); supportedDesktopFilePattern != want {
		t.Errorf("native picker pattern = %q, want %q", supportedDesktopFilePattern, want)
	}

	for _, ext := range []string{".mflac", ".mgg"} {
		if _, ok := supportedDesktopExts[ext]; !ok {
			t.Errorf("required desktop extension %q is unsupported", ext)
		}
	}
}
