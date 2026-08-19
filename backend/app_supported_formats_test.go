package main

import "testing"

func TestDesktopSupportedExtensionListMatchesFilterMap(t *testing.T) {
	if len(supportedDesktopExtList) != len(supportedDesktopExts) {
		t.Fatalf("extension list has %d items, map has %d", len(supportedDesktopExtList), len(supportedDesktopExts))
	}
	for _, ext := range supportedDesktopExtList {
		if _, ok := supportedDesktopExts[ext]; !ok {
			t.Errorf("extension %q is listed but missing from filter map", ext)
		}
	}
}
