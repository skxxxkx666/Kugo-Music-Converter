package handler

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kugo-music-converter/internal/service"
)

const previewRetention = 24 * time.Hour

var previewAllowedExt = map[string]struct{}{
	".mp3":  {},
	".flac": {},
	".wav":  {},
	".ogg":  {},
	".m4a":  {},
	".aac":  {},
	".opus": {},
	".wma":  {},
}

func normalizePathKey(path string) string {
	return strings.ToLower(filepath.Clean(path))
}

func resolveRealPath(path string) (string, error) {
	absPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", err
	}
	return filepath.Clean(realPath), nil
}

func (h *ConvertHandler) registerPreviewFiles(results []service.BatchFileDoneEvent) {
	if len(results) == 0 {
		return
	}

	now := time.Now()
	h.previewMu.Lock()
	defer h.previewMu.Unlock()

	for key, ts := range h.previewFiles {
		if now.Sub(ts) > previewRetention {
			delete(h.previewFiles, key)
		}
	}

	for _, item := range results {
		if item.Status != "ok" {
			continue
		}
		outputPath := strings.TrimSpace(item.Output)
		if outputPath == "" {
			continue
		}
		abs, err := filepath.Abs(outputPath)
		if err != nil {
			continue
		}
		realPath, err := resolveRealPath(abs)
		if err != nil {
			continue
		}
		h.previewFiles[normalizePathKey(realPath)] = now
	}
}

func (h *ConvertHandler) canPreviewFile(path string) bool {
	realPath, err := resolveRealPath(path)
	if err != nil {
		return false
	}
	key := normalizePathKey(realPath)
	now := time.Now()

	h.previewMu.Lock()
	defer h.previewMu.Unlock()

	ts, ok := h.previewFiles[key]
	if !ok {
		return false
	}
	if now.Sub(ts) > previewRetention {
		delete(h.previewFiles, key)
		return false
	}
	return true
}

func (h *ConvertHandler) HandlePreviewFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	rawPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if rawPath == "" {
		writeError(w, http.StatusBadRequest, NewAppError("ERR_UNKNOWN", "缺少试听文件路径", nil))
		return
	}

	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, NewAppError("ERR_UNKNOWN", "试听文件路径无效", err))
		return
	}
	realPath, err := resolveRealPath(absPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, NewAppError("ERR_UNKNOWN", "试听文件路径解析失败", err))
		return
	}

	ext := strings.ToLower(filepath.Ext(realPath))
	if _, ok := previewAllowedExt[ext]; !ok {
		writeError(w, http.StatusBadRequest, NewAppError(ErrUnsupportedFormat, "不支持试听该格式文件", nil))
		return
	}
	if !h.canPreviewFile(realPath) {
		writeError(w, http.StatusForbidden, NewAppError(ErrInputPathDenied, "该文件不在允许试听的转换结果中", nil))
		return
	}

	file, err := os.Open(realPath)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeError(w, status, NewAppError("ERR_UNKNOWN", "试听文件不可用", err))
		return
	}
	defer file.Close()

	st, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, NewAppError("ERR_UNKNOWN", "读取试听文件失败", err))
		return
	}
	if !st.Mode().IsRegular() {
		writeError(w, http.StatusBadRequest, NewAppError("ERR_UNKNOWN", "试听目标不是文件", nil))
		return
	}

	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, filepath.Base(realPath), st.ModTime(), file)
}
