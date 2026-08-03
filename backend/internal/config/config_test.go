package config

import "testing"

func TestDefaultConfigAllowsLargeAudioFiles(t *testing.T) {
	cfg := DefaultConfig()

	const want = int64(1 << 30)
	if cfg.MaxFileSize != want {
		t.Fatalf("expected default max file size %d, got %d", want, cfg.MaxFileSize)
	}
	if cfg.MaxFileSize <= 200<<20 {
		t.Fatalf("default max file size must allow files larger than 200 MiB")
	}
}
