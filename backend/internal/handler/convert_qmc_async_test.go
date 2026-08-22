package handler

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kugo-music-converter/internal/config"
	"kugo-music-converter/internal/qmckey"
	"kugo-music-converter/internal/service"
	"unlock-music.dev/cli/algo/qmc"
)

type blockingQMCResolver struct {
	started chan struct{}
	release chan struct{}
}

func (r *blockingQMCResolver) Resolve(ctx context.Context, resource qmckey.Resource) (string, error) {
	results := r.ResolveBatch(ctx, []qmckey.Resource{resource})
	if len(results) == 0 {
		return "", qmckey.ErrProtocol
	}
	return results[0].EKey, results[0].Err
}

func (r *blockingQMCResolver) ResolveBatch(ctx context.Context, resources []qmckey.Resource) []qmckey.BatchResult {
	select {
	case <-r.started:
	default:
		close(r.started)
	}
	select {
	case <-ctx.Done():
		results := make([]qmckey.BatchResult, len(resources))
		for index, resource := range resources {
			results[index] = qmckey.BatchResult{Resource: resource, Err: ctx.Err()}
		}
		return results
	case <-r.release:
		results := make([]qmckey.BatchResult, len(resources))
		for index, resource := range resources {
			results[index] = qmckey.BatchResult{Resource: resource, Err: qmckey.ErrNetwork}
		}
		return results
	}
}

func TestExecuteBatchDoesNotBlockOfflineItemsOnQMCKeyFetch(t *testing.T) {
	root := t.TempDir()
	modernPath := filepath.Join(root, "modern.mgg")
	modern := append([]byte("encrypted"), testMusicExFooter(t, "ModernMID", "O8M000Modern.mgg")...)
	if err := os.WriteFile(modernPath, modern, 0o600); err != nil {
		t.Fatalf("WriteFile(modern) error = %v", err)
	}

	plain := append([]byte("ID3"), bytes.Repeat([]byte{0x31}, 4096)...)
	encrypted := append([]byte(nil), plain...)
	cipher, err := qmc.NewQmcCipherDecoder(nil)
	if err != nil {
		t.Fatalf("NewQmcCipherDecoder() error = %v", err)
	}
	cipher.Decrypt(encrypted, 0)
	legacyPath := filepath.Join(root, "offline.qmc0")
	if err := os.WriteFile(legacyPath, encrypted, 0o600); err != nil {
		t.Fatalf("WriteFile(legacy) error = %v", err)
	}

	resolver := &blockingQMCResolver{started: make(chan struct{}), release: make(chan struct{})}
	cfg := config.DefaultConfig()
	handler := &ConvertHandler{
		cfg:            cfg,
		decryptService: service.NewDecryptService(cfg),
		qmcKeyResolver: resolver,
		baseDir:        root,
		shutdownCtx:    context.Background(),
	}
	req := &convertRequest{
		Items: []service.BatchItem{
			{Path: modernPath, OriginPath: modernPath, Name: filepath.Base(modernPath), Size: int64(len(modern)), Current: 1},
			{Path: legacyPath, OriginPath: legacyPath, Name: filepath.Base(legacyPath), Size: int64(len(encrypted)), Current: 2},
		},
		OutputDir: root, OutputFormat: "copy", MP3Quality: 2, Concurrency: 2, Cleanup: func() {},
	}

	offlineDone := make(chan struct{}, 1)
	summaryDone := make(chan service.BatchSummary, 1)
	go func() {
		summaryDone <- handler.executeBatch(context.Background(), req, func() bool { return false }, func(name string, payload any) {
			if name == "file-done" {
				if event, ok := payload.(service.BatchFileDoneEvent); ok && event.File == filepath.Base(legacyPath) && event.Status == "ok" {
					offlineDone <- struct{}{}
				}
			}
		})
	}()

	select {
	case <-resolver.started:
	case <-time.After(2 * time.Second):
		t.Fatal("QMC resolver did not start")
	}
	select {
	case <-offlineDone:
	case <-time.After(2 * time.Second):
		t.Fatal("offline QMC item remained blocked on modern QMC key fetch")
	}
	close(resolver.release)

	select {
	case summary := <-summaryDone:
		if summary.Success != 1 || summary.Failed != 1 {
			t.Fatalf("summary = %#v", summary)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("batch did not finish after resolver release")
	}
}
