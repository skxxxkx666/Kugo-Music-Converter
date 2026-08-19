package webview2bundle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var ErrPayloadUnavailable = errors.New("webview2 fixed runtime payload is unavailable")

var runtimeVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)

var (
	expandRuntimeCAB   = expandRuntimeCABWithSystemTool
	grantRuntimeAccess = configureRuntimeAccess
)

type PayloadInfo struct {
	CAB            []byte
	Version        string
	ExpectedSHA256 string
}

type EnsureResult struct {
	BrowserPath string
	Extracted   bool
}

func EnsureRuntime(cacheRoot string, info PayloadInfo) (EnsureResult, error) {
	if len(info.CAB) == 0 {
		return EnsureResult{}, ErrPayloadUnavailable
	}
	if strings.TrimSpace(cacheRoot) == "" {
		return EnsureResult{}, errors.New("webview2 cache root is empty")
	}

	version := strings.TrimSpace(info.Version)
	if !runtimeVersionPattern.MatchString(version) {
		return EnsureResult{}, fmt.Errorf("invalid webview2 fixed runtime version: %q", version)
	}
	expectedHash, err := normalizeSHA256(info.ExpectedSHA256)
	if err != nil {
		return EnsureResult{}, err
	}
	actualHash := sha256.Sum256(info.CAB)
	if hex.EncodeToString(actualHash[:]) != expectedHash {
		return EnsureResult{}, errors.New("webview2 fixed runtime payload checksum mismatch")
	}

	runtimeRoot := filepath.Join(cacheRoot, "webview2")
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		return EnsureResult{}, fmt.Errorf("create webview2 cache root: %w", err)
	}
	runtimeDir := filepath.Join(runtimeRoot, version+"-"+expectedHash[:16])
	if browserPath, ok := reusableRuntime(runtimeDir, version, expectedHash); ok {
		if err := ensureRuntimeAccess(runtimeDir, version, expectedHash); err != nil {
			return EnsureResult{}, err
		}
		return EnsureResult{BrowserPath: browserPath}, nil
	}

	stagingDir, err := os.MkdirTemp(runtimeRoot, ".extract-")
	if err != nil {
		return EnsureResult{}, fmt.Errorf("create webview2 staging directory: %w", err)
	}
	keepStaging := false
	defer func() {
		if !keepStaging {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	cabPath := filepath.Join(stagingDir, "webview2-runtime.cab")
	if err := os.WriteFile(cabPath, info.CAB, 0o600); err != nil {
		return EnsureResult{}, fmt.Errorf("write webview2 runtime payload: %w", err)
	}
	if err := expandRuntimeCAB(cabPath, stagingDir); err != nil {
		return EnsureResult{}, err
	}
	if err := os.Remove(cabPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return EnsureResult{}, fmt.Errorf("remove webview2 runtime payload: %w", err)
	}

	browserPath, err := findBrowserPath(stagingDir)
	if err != nil {
		return EnsureResult{}, err
	}
	relativeBrowserPath, err := filepath.Rel(stagingDir, browserPath)
	if err != nil || relativeBrowserPath == "." || strings.HasPrefix(relativeBrowserPath, "..") {
		return EnsureResult{}, errors.New("webview2 runtime browser path escaped staging directory")
	}
	if err := writeMarker(stagingDir, version, expectedHash); err != nil {
		return EnsureResult{}, err
	}

	if err := os.RemoveAll(runtimeDir); err != nil {
		return EnsureResult{}, fmt.Errorf("replace invalid webview2 runtime cache: %w", err)
	}
	if err := os.Rename(stagingDir, runtimeDir); err != nil {
		if existingPath, ok := reusableRuntime(runtimeDir, version, expectedHash); ok {
			if accessErr := ensureRuntimeAccess(runtimeDir, version, expectedHash); accessErr != nil {
				return EnsureResult{}, accessErr
			}
			return EnsureResult{BrowserPath: existingPath}, nil
		}
		return EnsureResult{}, fmt.Errorf("activate webview2 fixed runtime: %w", err)
	}
	keepStaging = true

	if err := ensureRuntimeAccess(runtimeDir, version, expectedHash); err != nil {
		return EnsureResult{}, err
	}
	return EnsureResult{
		BrowserPath: filepath.Join(runtimeDir, relativeBrowserPath),
		Extracted:   true,
	}, nil
}

func expandRuntimeCABWithSystemTool(cabPath string, destination string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	command := exec.CommandContext(ctx, "expand.exe", cabPath, "-F:*", destination)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return errors.New("webview2 fixed runtime extraction timed out")
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("extract webview2 fixed runtime: %s", message)
	}
	return nil
}

func reusableRuntime(runtimeDir string, version string, expectedHash string) (string, bool) {
	marker, err := os.ReadFile(filepath.Join(runtimeDir, ".runtime-ready"))
	if err != nil || string(marker) != version+"\n"+expectedHash+"\n" {
		return "", false
	}
	browserPath, err := findBrowserPath(runtimeDir)
	return browserPath, err == nil
}

func findBrowserPath(root string) (string, error) {
	var browserPath string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "msedgewebview2.exe") {
			browserPath = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("inspect webview2 fixed runtime: %w", err)
	}
	if browserPath == "" {
		return "", errors.New("webview2 fixed runtime does not contain msedgewebview2.exe")
	}
	return browserPath, nil
}

func writeMarker(runtimeDir string, version string, expectedHash string) error {
	marker := version + "\n" + expectedHash + "\n"
	if err := os.WriteFile(filepath.Join(runtimeDir, ".runtime-ready"), []byte(marker), 0o600); err != nil {
		return fmt.Errorf("write webview2 runtime marker: %w", err)
	}
	return nil
}

func ensureRuntimeAccess(runtimeDir string, version string, expectedHash string) error {
	markerValue := version + "\n" + expectedHash + "\n"
	markerPath := filepath.Join(runtimeDir, ".access-ready")
	if marker, err := os.ReadFile(markerPath); err == nil && string(marker) == markerValue {
		return nil
	}
	if err := grantRuntimeAccess(runtimeDir); err != nil {
		return err
	}
	if err := os.WriteFile(markerPath, []byte(markerValue), 0o600); err != nil {
		return fmt.Errorf("write webview2 access marker: %w", err)
	}
	return nil
}

func normalizeSHA256(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if len(normalized) != sha256.Size*2 {
		return "", errors.New("webview2 runtime checksum must be a SHA256 value")
	}
	if _, err := hex.DecodeString(normalized); err != nil {
		return "", errors.New("webview2 runtime checksum must be a SHA256 value")
	}
	return normalized, nil
}
