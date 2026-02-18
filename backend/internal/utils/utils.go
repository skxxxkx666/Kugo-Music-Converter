// Utility helpers for Kugo Music Converter.
package utils

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

func RandHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func EnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func ReplaceExt(name, newExt string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return name + newExt
	}
	return strings.TrimSuffix(name, ext) + newExt
}

func FileSizeSafe(path string) int64 {
	if fi, err := os.Stat(path); err == nil {
		return fi.Size()
	}
	return 0
}
