package handler

import "testing"

func TestSupportedInputExtensionsIncludeV060Formats(t *testing.T) {
	for _, name := range []string{
		"song.kwm",
		"song.qmc0",
		"song.qmc2",
		"song.qmc3",
		"song.qmc4",
		"song.qmc6",
		"song.qmc8",
		"song.qmcflac",
		"song.qmcogg",
		"song.tkm",
	} {
		if !containsInputExt(name) {
			t.Errorf("containsInputExt(%q) = false", name)
		}
	}

	for _, name := range []string{"song.mflac", "song.mgg", "song.exe"} {
		if containsInputExt(name) {
			t.Errorf("containsInputExt(%q) = true", name)
		}
	}
}
