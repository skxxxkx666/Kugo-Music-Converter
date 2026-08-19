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
	ErrFFmpegUnavailable = "ERR_FFMPEG_UNAVAILABLE"
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
	ErrInputPathDenied   = "ERR_INPUT_PATH_DENIED"
	ErrForbiddenOrigin   = "ERR_FORBIDDEN_ORIGIN"
	// 以下三个为前端在 SSE 链路异常时合成的错误码，登记于此以保持错误码
	// 目录权威、文案统一（F-5008，落地 v0.4.1 RC-5008）。
	ErrBackendUnreachable = "ERR_BACKEND_UNREACHABLE"
	ErrStreamNoComplete   = "ERR_STREAM_NO_COMPLETE"
	ErrStreamDisconnected = "ERR_STREAM_DISCONNECTED" // 兼容旧码（过渡期保留）
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
	ErrDBNotFound:         {userMessage: "未找到 KGMusicV3.db 数据库文件。", suggestion: "KGG 格式转换需要数据库，请先配置 KGMusicV3.db。", severity: "fatal"},
	ErrDecryptFailed:      {userMessage: "解密失败，未生成可用音频文件。", suggestion: "请确认输入文件完整可用后重试。", severity: "error"},
	ErrDecryptKeyExpired:  {userMessage: "解密失败，密钥可能已失效。", suggestion: "请先在酷狗客户端播放一次该歌曲后重试。", severity: "error"},
	ErrTranscodeFailed:    {userMessage: "音频转码失败。", suggestion: "请确认输入文件完整可用，或更换输出格式后重试。", severity: "error"},
	ErrFFmpegUnavailable:  {userMessage: "运行时 FFmpeg 不可用。", suggestion: "请重启应用；若仍失败，请重新下载安装包或查看诊断日志。", severity: "fatal"},
	ErrUnsupportedFormat:  {userMessage: "不支持的输入文件格式。", suggestion: "支持 KGG/KGM/KGMA/VPR/NCM/KWM 及传统 QMC 格式。", severity: "warning"},
	ErrRuntimeMissing:     {userMessage: "运行时依赖缺失。", suggestion: "请重启应用；若仍失败，请重新下载安装包。", severity: "fatal"},
	ErrNoFiles:            {userMessage: "未选择任何支持的文件。", suggestion: "请先选择至少一个加密音频文件。", severity: "warning"},
	ErrTooManyFiles:       {userMessage: "文件数量超过限制。", suggestion: "请减少本次文件数量或分批处理。", severity: "warning"},
	ErrFileTooLarge:       {userMessage: "文件内容超过大小限制。", suggestion: "请检查单文件与本次任务总大小，必要时分批处理或使用文件夹扫描。", severity: "warning"},
	ErrOutputRequired:     {userMessage: "输出目录不能为空。", suggestion: "请先选择输出目录。", severity: "warning"},
	ErrFolderPicker:       {userMessage: "无法打开目录选择器。", suggestion: "请重试，或使用拖放方式添加文件夹。", severity: "error"},
	ErrDBPicker:           {userMessage: "无法打开数据库选择器。", suggestion: "请重试，并检查系统文件选择窗口是否被安全软件拦截。", severity: "error"},
	ErrDBPathInvalid:      {userMessage: "数据库路径无效。", suggestion: "请确认文件存在且文件名为 KGMusicV3.db。", severity: "warning"},
	ErrCancelled:          {userMessage: "转换已取消。", suggestion: "可重新发起转换任务。", severity: "warning"},
	ErrScanInvalidPath:    {userMessage: "扫描路径无效。", suggestion: "请确认路径存在且为文件夹。", severity: "warning"},
	ErrInputPathDenied:    {userMessage: "输入路径不受支持。", suggestion: "请选择可读取的本机磁盘绝对路径；暂不支持网络共享路径。", severity: "warning"},
	ErrForbiddenOrigin:    {userMessage: "请求来源不受信任。", suggestion: "请仅使用本项目提供的桌面应用或本机界面。", severity: "fatal"},
	ErrBackendUnreachable: {userMessage: "无法连接转换进程。", suggestion: "请重启应用；若仍失败，请检查安全软件是否拦截应用进程。", severity: "fatal"},
	ErrStreamNoComplete:   {userMessage: "转换连接中断，任务状态未知。", suggestion: "请检查本机安全软件或进程稳定性后，重试未完成文件。", severity: "error"},
	ErrStreamDisconnected: {userMessage: "转换连接中断，任务状态未知。", suggestion: "请检查本机安全软件或进程稳定性后，重试未完成文件。", severity: "error"},
}

func NewAppError(code string, detail string, inner error) *AppError {
	meta, ok := errorCatalog[code]
	if !ok {
		meta = errorMeta{
			userMessage: "发生未知错误。",
			suggestion:  "请查看日志后重试。",
			severity:    "error",
		}
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

func requestErrorStatus(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) && appErr.Code == ErrFileTooLarge {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func writeMethodNotAllowed(w http.ResponseWriter, allow string) {
	if allow != "" {
		w.Header().Set("Allow", allow)
	}
	writeError(w, http.StatusMethodNotAllowed, NewAppError("ERR_UNKNOWN", "method not allowed", nil))
}
