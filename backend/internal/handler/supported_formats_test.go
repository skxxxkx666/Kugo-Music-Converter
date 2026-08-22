package handler

import "testing"

func TestSupportedInputExtensionsKeepLegacyServerSurface(t *testing.T) {
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

	for _, name := range []string{"song.mflac", "song.mgg"} {
		if containsInputExt(name) {
			t.Errorf("legacy containsInputExt(%q) = true", name)
		}
		if !(&ConvertHandler{supportsModernQMC: true}).containsLocalInputExt(name) {
			t.Errorf("desktop containsLocalInputExt(%q) = false", name)
		}
	}

	if containsInputExt("song.exe") {
		t.Error("containsInputExt(\"song.exe\") = true")
	}
}
