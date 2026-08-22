package handler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"kugo-music-converter/internal/config"
	"kugo-music-converter/internal/service"
)

func TestExecuteBatchIsolatesKGGDatabaseFailure(t *testing.T) {
	root := t.TempDir()
	kggPath := filepath.Join(root, "missing-key.kgg")
	ncmPath := filepath.Join(root, "invalid.ncm")
	for _, path := range []string{kggPath, ncmPath} {
		if err := os.WriteFile(path, []byte("not encrypted audio"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
	}

	cfg := config.DefaultConfig()
	handler := &ConvertHandler{
		cfg:            cfg,
		decryptService: service.NewDecryptService(cfg),
		baseDir:        root,
		ffmpegPath:     filepath.Join(root, "unused-ffmpeg.exe"),
		shutdownCtx:    context.Background(),
	}
	req := &convertRequest{
		Items: []service.BatchItem{
			{Path: kggPath, OriginPath: kggPath, Name: filepath.Base(kggPath), Size: 19, Current: 1},
			{Path: ncmPath, OriginPath: ncmPath, Name: filepath.Base(ncmPath), Size: 19, Current: 2},
		},
		OutputDir:    root,
		DBPath:       filepath.Join(root, "does-not-exist.db"),
		OutputFormat: "copy",
		MP3Quality:   2,
		Concurrency:  1,
		Cleanup:      func() {},
	}

	summary := handler.executeBatch(context.Background(), req, func() bool { return false }, nil)
	if summary.Failed != 2 || len(summary.Results) != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if summary.Results[0].Error == nil || summary.Results[0].Error.Code != ErrDBNotFound {
		t.Fatalf("KGG error = %#v, want %s", summary.Results[0].Error, ErrDBNotFound)
	}
	if summary.Results[1].Error == nil || summary.Results[1].Error.Code == ErrDBNotFound {
		t.Fatalf("non-KGG error = %#v, must not inherit KGG database failure", summary.Results[1].Error)
	}
}
