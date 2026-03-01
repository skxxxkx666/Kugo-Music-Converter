package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	warnings := make([]string, 0, len(req.Paths))
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
			warnings = append(warnings, fmt.Sprintf("路径无效：%s", path))
			continue
		}
		st, err := os.Stat(abs)
		if err != nil || !st.IsDir() {
			warnings = append(warnings, fmt.Sprintf("目录不可访问：%s", abs))
			continue
		}

		files, size, err := service.ScanSingleFolderCtx(ctx, abs, req.Recursive, filter, remainingLimit)
		if len(files) > 0 {
			folders = append(folders, service.ScanFolderInfo{Path: abs, Files: files})
			totalFiles += len(files)
			totalSize += size
			remainingLimit -= len(files)
		}
		if err != nil {
			if errors.Is(err, service.ErrScanLimitReached) {
				warnings = append(warnings, fmt.Sprintf("扫描达到上限 %d 个文件，结果已截断。", scanMaxFiles))
				break
			}
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				warnings = append(warnings, fmt.Sprintf("扫描超时：%s", abs))
				break
			}
			warnings = append(warnings, fmt.Sprintf("扫描失败：%s", abs))
			continue
		}
		if remainingLimit <= 0 || ctx.Err() != nil {
			if ctx.Err() != nil {
				warnings = append(warnings, "扫描已提前终止（请求取消或超时）。")
			}
			break
		}
	}

	writeJSON(w, http.StatusOK, service.ScanResult{
		TotalFiles: totalFiles,
		TotalSize:  totalSize,
		Folders:    folders,
		Warnings:   warnings,
	})
}
