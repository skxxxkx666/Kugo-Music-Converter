package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const (
	ErrDBNotFound        = "ERR_DB_NOT_FOUND"
	ErrDecryptFailed     = "ERR_DECRYPT_FAILED"
	ErrDecryptKeyExpired = "ERR_DECRYPT_KEY_EXPIRED"
	ErrTranscodeFailed   = "ERR_TRANSCODE_FAILED"
	ErrUnsupportedFormat = "ERR_UNSUPPORTED_FORMAT"
	ErrRuntimeMissing    = "ERR_RUNTIME_MISSING"
	ErrNoFiles           = "ERR_NO_FILES"
	ErrTooManyFiles      = "ERR_TOO_MANY_FILES"
	ErrFileTooLarge      = "ERR_FILE_TOO_LARGE"
	ErrOutputRequired    = "ERR_OUTPUT_REQUIRED"
	ErrFolderPicker      = "ERR_FOLDER_PICKER"
	ErrDBPicker          = "ERR_DB_PICKER"
	ErrDBPathInvalid     = "ERR_DB_PATH_INVALID"
	ErrCancelled         = "ERR_CANCELLED"
	ErrScanInvalidPath   = "ERR_SCAN_INVALID_PATH"
)

type AppError struct {
	Code        string `json:"code"`
	UserMessage string `json:"userMessage"`
	Suggestion  string `json:"suggestion,omitempty"`
	Severity    string `json:"severity"`
	Detail      string `json:"detail,omitempty"`
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Detail) != "" {
		return e.Detail
	}
	return e.UserMessage
}

type ErrorResponse struct {
	Success     bool      `json:"success"`
	Error       *AppError `json:"error"`
	Code        string    `json:"code"`
	UserMessage string    `json:"userMessage"`
	Suggestion  string    `json:"suggestion,omitempty"`
	Severity    string    `json:"severity"`
}

type errorMeta struct {
	userMessage string
	suggestion  string
	severity    string
}

var errorCatalog = map[string]errorMeta{
	ErrDBNotFound:        {"未找到 KGMusicV3.db 数据库文件。", "仅 KGG 格式需要数据库，请先配置 KGMusicV3.db。", "fatal"},
	ErrDecryptFailed:     {"解密失败，未生成可用音频文件。", "请确认输入文件完整可用后重试。", "error"},
	ErrDecryptKeyExpired: {"解密失败，密钥可能已失效。", "请先在酷狗客户端播放一次该歌曲后重试。", "error"},
	ErrTranscodeFailed:   {"音频转码失败。", "请确认 ffmpeg 可用，或尝试更换输入文件后重试。", "error"},
	ErrUnsupportedFormat: {"不支持的输入文件格式。", "仅支持 .kgg/.kgm/.kgma/.vpr/.ncm。", "warning"},
	ErrRuntimeMissing:    {"运行时依赖缺失。", "请补齐缺失文件后重试。", "fatal"},
	ErrNoFiles:           {"未上传任何支持的文件。", "请先选择至少一个加密音频文件。", "warning"},
	ErrTooManyFiles:      {"上传文件数量超过限制。", "请分批上传。", "warning"},
	ErrFileTooLarge:      {"单文件超过大小限制。", "请减小文件大小后重试。", "warning"},
	ErrOutputRequired:    {"输出目录不能为空。", "请先选择输出目录。", "warning"},
	ErrFolderPicker:      {"无法打开目录选择器。", "请手动输入目录路径。", "error"},
	ErrDBPicker:          {"无法打开数据库选择器。", "请手动输入 KGMusicV3.db 路径。", "error"},
	ErrDBPathInvalid:     {"数据库路径无效。", "请确认文件存在且文件名为 KGMusicV3.db。", "warning"},
	ErrCancelled:         {"转换已取消。", "可重新发起转换任务。", "warning"},
	ErrScanInvalidPath:   {"扫描路径无效。", "请确认路径存在且为文件夹。", "warning"},
}

func NewAppError(code string, detail string, inner error) *AppError {
	meta, ok := errorCatalog[code]
	if !ok {
		meta = errorMeta{userMessage: "发生未知错误。", suggestion: "请查看日志后重试。", severity: "error"}
		code = "ERR_UNKNOWN"
	}

	if detail == "" && inner != nil {
		detail = inner.Error()
	}
	if detail == "" {
		detail = meta.userMessage
	}

	return &AppError{
		Code:        code,
		UserMessage: meta.userMessage,
		Suggestion:  meta.suggestion,
		Severity:    meta.severity,
		Detail:      detail,
	}
}

func asAppError(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return NewAppError("ERR_UNKNOWN", err.Error(), err)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	appErr := asAppError(err)
	writeJSON(w, status, ErrorResponse{
		Success:     false,
		Error:       appErr,
		Code:        appErr.Code,
		UserMessage: appErr.UserMessage,
		Suggestion:  appErr.Suggestion,
		Severity:    appErr.Severity,
	})
}
