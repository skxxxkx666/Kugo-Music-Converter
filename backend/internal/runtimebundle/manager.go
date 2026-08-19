package runtimebundle

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrPayloadUnavailable = errors.New("ffmpeg runtime payload is unavailable")

type EnsureResult struct {
	Path      string
	Extracted bool
}

func EnsureFFmpeg(cacheRoot string, payload []byte, expectedSHA256 string) (EnsureResult, error) {
	if len(payload) == 0 {
		return EnsureResult{}, ErrPayloadUnavailable
	}

	expectedHash, err := normalizeSHA256(expectedSHA256)
	if err != nil {
		return EnsureResult{}, err
	}
	if strings.TrimSpace(cacheRoot) == "" {
		return EnsureResult{}, errors.New("runtime cache root is empty")
	}

	runtimeDir := filepath.Join(cacheRoot, "runtime", expectedHash[:16])
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return EnsureResult{}, fmt.Errorf("create runtime cache: %w", err)
	}

	targetPath := filepath.Join(runtimeDir, "ffmpeg.exe")
	if fileMatchesSHA256(targetPath, expectedHash) {
		return EnsureResult{Path: targetPath}, nil
	}

	tempFile, err := os.CreateTemp(runtimeDir, "ffmpeg-*.tmp")
	if err != nil {
		return EnsureResult{}, fmt.Errorf("create runtime temp file: %w", err)
	}
	tempPath := tempFile.Name()
	keepTemp := false
	defer func() {
		_ = tempFile.Close()
		if !keepTemp {
			_ = os.Remove(tempPath)
		}
	}()

	gzipReader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return EnsureResult{}, fmt.Errorf("open ffmpeg payload: %w", err)
	}

	hasher := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(tempFile, hasher), gzipReader)
	closeGzipErr := gzipReader.Close()
	if copyErr != nil {
		return EnsureResult{}, fmt.Errorf("extract ffmpeg payload: %w", copyErr)
	}
	if closeGzipErr != nil {
		return EnsureResult{}, fmt.Errorf("close ffmpeg payload: %w", closeGzipErr)
	}
	if err := tempFile.Sync(); err != nil {
		return EnsureResult{}, fmt.Errorf("flush ffmpeg runtime: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return EnsureResult{}, fmt.Errorf("close ffmpeg runtime: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedHash {
		return EnsureResult{}, fmt.Errorf("ffmpeg runtime checksum mismatch: got %s", actualHash)
	}

	_ = os.Remove(targetPath)
	if err := os.Rename(tempPath, targetPath); err != nil {
		if fileMatchesSHA256(targetPath, expectedHash) {
			return EnsureResult{Path: targetPath}, nil
		}
		return EnsureResult{}, fmt.Errorf("activate ffmpeg runtime: %w", err)
	}
	keepTemp = true

	return EnsureResult{Path: targetPath, Extracted: true}, nil
}

func normalizeSHA256(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if len(normalized) != sha256.Size*2 {
		return "", errors.New("ffmpeg runtime checksum must be a SHA256 value")
	}
	if _, err := hex.DecodeString(normalized); err != nil {
		return "", errors.New("ffmpeg runtime checksum must be a SHA256 value")
	}
	return normalized, nil
}

func fileMatchesSHA256(path string, expected string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}
	return hex.EncodeToString(hasher.Sum(nil)) == expected
}
