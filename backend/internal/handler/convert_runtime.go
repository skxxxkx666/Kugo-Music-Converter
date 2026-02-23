package handler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"kugo-music-converter/internal/service"
)

func containsInputExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, item := range supportedInputExts {
		if ext == item {
			return true
		}
	}
	return false
}

func normalizeConcurrency(raw int, fallback int) int {
	if raw <= 0 {
		raw = fallback
	}
	if raw <= 0 {
		raw = runtimeDefaultConcurrency()
	}
	if raw < minConcurrency {
		raw = minConcurrency
	}
	if raw > runtimeMaxConcurrency() {
		raw = runtimeMaxConcurrency()
	}
	return raw
}

func runtimeMaxConcurrency() int {
	max := runtime.NumCPU()
	if max <= 0 {
		max = minConcurrency
	}
	if max > maxConcurrencyHardCap {
		max = maxConcurrencyHardCap
	}
	if max < minConcurrency {
		max = minConcurrency
	}
	return max
}

func runtimeDefaultConcurrency() int {
	defaultValue := runtime.NumCPU() / 2
	if defaultValue < minConcurrency {
		defaultValue = minConcurrency
	}
	if defaultValue > runtimeMaxConcurrency() {
		defaultValue = runtimeMaxConcurrency()
	}
	return defaultValue
}

func (h *ConvertHandler) runtimeMissingTools() []string {
	missing := make([]string, 0, 1)
	if st, err := os.Stat(h.ffmpegPath); err != nil || st.IsDir() {
		missing = append(missing, "tools/ffmpeg.exe")
	}
	return missing
}

func detectErrorCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ErrCancelled
	case errors.Is(err, service.ErrUnsupportedInput):
		return ErrUnsupportedFormat
	case errors.Is(err, service.ErrTranscodeProcess):
		return ErrTranscodeFailed
	case errors.Is(err, service.ErrMissingKGGKey):
		return ErrDecryptKeyExpired
	case errors.Is(err, service.ErrUnknownAudio), errors.Is(err, service.ErrDecryptProcess):
		return ErrDecryptFailed
	default:
		return ErrDecryptFailed
	}
}

func toBatchFileError(err error) *service.BatchFileError {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return &service.BatchFileError{
			Code:        appErr.Code,
			UserMessage: appErr.UserMessage,
			Suggestion:  appErr.Suggestion,
			Severity:    appErr.Severity,
			Detail:      appErr.Detail,
		}
	}

	mapped := NewAppError(detectErrorCode(err), err.Error(), nil)
	return &service.BatchFileError{
		Code:        mapped.Code,
		UserMessage: mapped.UserMessage,
		Suggestion:  mapped.Suggestion,
		Severity:    mapped.Severity,
		Detail:      mapped.Detail,
	}
}
