package runtimebundle

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"testing"
)

func TestEnsureFFmpegExtractsAndReusesPayload(t *testing.T) {
	raw := bytes.Repeat([]byte("fake-ffmpeg-runtime"), 128)
	payload := gzipPayload(t, raw)
	hash := sha256Hex(raw)

	first, err := EnsureFFmpeg(t.TempDir(), payload, hash)
	if err != nil {
		t.Fatalf("EnsureFFmpeg() first call error = %v", err)
	}
	if !first.Extracted {
		t.Fatal("EnsureFFmpeg() first call did not report extraction")
	}
	got, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatal("extracted runtime does not match payload")
	}
}

func TestEnsureFFmpegReusesSameCache(t *testing.T) {
	raw := []byte("fake-ffmpeg-runtime")
	payload := gzipPayload(t, raw)
	cacheRoot := t.TempDir()

	first, err := EnsureFFmpeg(cacheRoot, payload, sha256Hex(raw))
	if err != nil {
		t.Fatalf("EnsureFFmpeg() first call error = %v", err)
	}
	second, err := EnsureFFmpeg(cacheRoot, payload, sha256Hex(raw))
	if err != nil {
		t.Fatalf("EnsureFFmpeg() second call error = %v", err)
	}
	if second.Extracted {
		t.Fatal("EnsureFFmpeg() extracted an already valid cached runtime")
	}
	if second.Path != first.Path {
		t.Fatalf("cached path = %q, want %q", second.Path, first.Path)
	}
}

func TestEnsureFFmpegRejectsChecksumMismatch(t *testing.T) {
	raw := []byte("fake-ffmpeg-runtime")
	otherHash := sha256Hex([]byte("different-runtime"))

	_, err := EnsureFFmpeg(t.TempDir(), gzipPayload(t, raw), otherHash)
	if err == nil {
		t.Fatal("EnsureFFmpeg() error = nil, want checksum mismatch")
	}
}

func TestEnsureFFmpegRequiresPayload(t *testing.T) {
	_, err := EnsureFFmpeg(t.TempDir(), nil, sha256Hex([]byte("unused")))
	if !errors.Is(err, ErrPayloadUnavailable) {
		t.Fatalf("EnsureFFmpeg() error = %v, want ErrPayloadUnavailable", err)
	}
}

func gzipPayload(t *testing.T, raw []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(raw); err != nil {
		t.Fatalf("gzip.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("gzip.Close() error = %v", err)
	}
	return compressed.Bytes()
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
