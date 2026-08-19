//go:build !bindings

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"kugo-music-converter/internal/webview2bundle"
)

func prepareWebView2Runtime() (string, error) {
	payload := webview2bundle.EmbeddedPayload()
	if len(payload.CAB) == 0 {
		return "", nil
	}

	cacheDirectory, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("定位用户缓存目录: %w", err)
	}
	result, err := webview2bundle.EnsureRuntime(
		filepath.Join(cacheDirectory, "Kugo Music Converter"),
		payload,
	)
	if err != nil {
		return "", err
	}
	return result.BrowserPath, nil
}
