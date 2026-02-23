package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kugo-music-converter/internal/service"
)

const (
	scanTimeout  = 30 * time.Second
	scanMaxFiles = 50000
)

type scanRequest struct {
	Paths     []string `json:"paths"`
	Recursive bool     `json:"recursive"`
	Filter    string   `json:"filter"`
}

func (h *ConvertHandler) HandleScanFolders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var req scanRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, NewAppError(ErrScanInvalidPath, "请求体格式错误", err))
		return
	}

	if req.Paths == nil {
		req.Paths = []string{}
	}

	filter := service.ParseExtFilter(req.Filter)
	folders := make([]service.ScanFolderInfo, 0, len(req.Paths))
	totalFiles := 0
	var totalSize int64
	remainingLimit := scanMaxFiles

	ctx, cancel := context.WithTimeout(r.Context(), scanTimeout)
	defer cancel()

	for _, rawPath := range req.Paths {
		path := strings.TrimSpace(rawPath)
		if path == "" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		st, err := os.Stat(abs)
		if err != nil || !st.IsDir() {
			continue
		}

		files, size, err := service.ScanSingleFolderCtx(ctx, abs, req.Recursive, filter, remainingLimit)
		if err != nil {
			continue
		}

		folders = append(folders, service.ScanFolderInfo{Path: abs, Files: files})
		totalFiles += len(files)
		totalSize += size
		remainingLimit -= len(files)
		if remainingLimit <= 0 || ctx.Err() != nil {
			break
		}
	}

	writeJSON(w, http.StatusOK, service.ScanResult{TotalFiles: totalFiles, TotalSize: totalSize, Folders: folders})
}
