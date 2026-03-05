package handler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"kugo-music-converter/internal/service"
)

const (
	ffmpegProbeTimeout  = 8 * time.Second
	ffmpegProbeCacheTTL = 3 * time.Second
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

func compactMessage(raw string, maxLen int) string {
	msg := strings.TrimSpace(strings.ReplaceAll(raw, "\r", ""))
	msg = strings.ReplaceAll(msg, "\n", " | ")
	if maxLen > 0 && len(msg) > maxLen {
		return strings.TrimSpace(msg[:maxLen]) + "..."
	}
	return msg
}

func probeFFmpegBinary(ffmpegPath string) (bool, string) {
	path := strings.TrimSpace(ffmpegPath)
	if path == "" {
		return false, "未配置 ffmpeg 路径"
	}
	st, err := os.Stat(path)
	if err != nil {
		return false, fmt.Sprintf("ffmpeg 文件不存在: %v", err)
	}
	if st.IsDir() {
		return false, "ffmpeg 路径指向目录，无法执行"
	}
	if runtime.GOOS != "windows" && (st.Mode()&0o111) == 0 {
		return false, "ffmpeg 缺少可执行权限"
	}

	ctx, cancel := context.WithTimeout(context.Background(), ffmpegProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-version")
	output, runErr := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return false, "ffmpeg 探测超时（2s）"
	}
	if runErr != nil {
		msg := compactMessage(string(output), 200)
		if msg == "" {
			msg = runErr.Error()
		}
		return false, fmt.Sprintf("ffmpeg 执行失败: %s", msg)
	}

	msg := compactMessage(string(output), 200)
	if msg == "" {
		msg = "ffmpeg 可用"
	}
	return true, msg
}

func ffmpegBinaryPresent(ffmpegPath string) (bool, string) {
	path := strings.TrimSpace(ffmpegPath)
	if path == "" {
		return false, "未配置 ffmpeg 路径"
	}
	st, err := os.Stat(path)
	if err != nil {
		return false, fmt.Sprintf("ffmpeg 文件不存在: %v", err)
	}
	if st.IsDir() {
		return false, "ffmpeg 路径指向目录，无法执行"
	}
	if runtime.GOOS != "windows" && (st.Mode()&0o111) == 0 {
		return false, "ffmpeg 缺少可执行权限"
	}
	return true, "ffmpeg 文件存在"
}

func (h *ConvertHandler) probeFFmpeg(force bool) (bool, string) {
	h.ffmpegProbeMu.Lock()
	defer h.ffmpegProbeMu.Unlock()

	if !force && !h.ffmpegCheckedAt.IsZero() && time.Since(h.ffmpegCheckedAt) < ffmpegProbeCacheTTL {
		return h.ffmpegReady, h.ffmpegMessage
	}

	ready, message := probeFFmpegBinary(h.ffmpegPath)
	h.ffmpegReady = ready
	h.ffmpegMessage = message
	h.ffmpegCheckedAt = time.Now()
	return ready, message
}

func (h *ConvertHandler) runtimeMissingTools() []string {
	missing := make([]string, 0, 1)
	ready, _ := ffmpegBinaryPresent(h.ffmpegPath)
	if !ready {
		missing = append(missing, "tools/ffmpeg.exe")
	}
	return missing
}

func containsAnyToken(message string, tokens []string) bool {
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.Contains(message, token) {
			return true
		}
	}
	return false
}

func (h *ConvertHandler) classifyTranscodeFailure(detail string) string {
	lower := strings.ToLower(strings.TrimSpace(detail))

	ffmpegHints := []string{
		"executable file not found",
		"no such file or directory",
		"cannot find the file",
		"is not recognized",
		"permission denied",
		"access is denied",
		"exec format error",
		"fork/exec",
		"createprocess",
	}
	inputHints := []string{
		"invalid data",
		"invalid argument",
		"invalid audio",
		"error while decoding",
		"conversion failed",
		"header missing",
		"moov atom not found",
		"corrupt",
		"unsupported",
	}

	switch {
	case containsAnyToken(lower, ffmpegHints):
		return ErrFFmpegUnavailable
	case containsAnyToken(lower, inputHints):
		return ErrTranscodeFailed
	default:
		ready, _ := h.probeFFmpeg(false)
		if !ready {
			return ErrFFmpegUnavailable
		}
		return ErrTranscodeFailed
	}
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
