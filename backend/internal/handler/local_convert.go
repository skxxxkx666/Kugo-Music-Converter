package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kugo-music-converter/internal/service"
)

type LocalConversionRequest struct {
	Paths        []string `json:"paths"`
	OutputDir    string   `json:"outputDir"`
	DBPath       string   `json:"dbPath,omitempty"`
	OutputFormat string   `json:"outputFormat"`
	MP3Quality   int      `json:"mp3Quality"`
	Concurrency  int      `json:"concurrency"`
}

func (h *ConvertHandler) DatabaseStatus() service.DBStatus {
	return h.getDBStatus()
}

func (h *ConvertHandler) RedetectDatabase() service.DBStatus {
	if h == nil {
		return service.DBStatus{Source: "missing"}
	}
	status := service.DetectKGMusicDB(h.baseDir)
	if status.Found {
		if err := h.loadDBByPath(status.Path, status.Source); err == nil {
			return h.getDBStatus()
		}
		return service.DBStatus{Source: "missing"}
	}
	if current := h.getDBStatus(); current.Found {
		return current
	}
	return status
}

func (h *ConvertHandler) ValidateLocalConversion(input LocalConversionRequest) error {
	if h == nil || h.cfg == nil {
		return NewAppError(ErrRuntimeMissing, "转换服务尚未初始化", nil)
	}
	if missing := h.runtimeMissingTools(); len(missing) > 0 {
		return NewAppError(ErrRuntimeMissing, strings.Join(missing, ","), nil)
	}
	_, err := h.buildLocalConvertRequest(input)
	return err
}

func (h *ConvertHandler) ConvertLocalPaths(
	ctx context.Context,
	input LocalConversionRequest,
	onEvent func(name string, payload any),
) (service.BatchSummary, error) {
	if h == nil || h.cfg == nil {
		return service.BatchSummary{}, NewAppError(ErrRuntimeMissing, "转换服务尚未初始化", nil)
	}
	if missing := h.runtimeMissingTools(); len(missing) > 0 {
		return service.BatchSummary{}, NewAppError(ErrRuntimeMissing, strings.Join(missing, ","), nil)
	}
	req, err := h.buildLocalConvertRequest(input)
	if err != nil {
		return service.BatchSummary{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	summary := h.executeBatch(ctx, req, func() bool {
		return ctx.Err() != nil
	}, onEvent)
	return summary, nil
}

func (h *ConvertHandler) buildLocalConvertRequest(input LocalConversionRequest) (*convertRequest, error) {
	if h == nil || h.cfg == nil {
		return nil, NewAppError(ErrRuntimeMissing, "转换服务尚未初始化", nil)
	}
	if len(input.Paths) == 0 {
		return nil, NewAppError(ErrNoFiles, "未选择可转换文件", nil)
	}
	if len(input.Paths) > h.cfg.MaxFiles {
		return nil, NewAppError(ErrTooManyFiles, fmt.Sprintf("文件数量超过限制（最多 %d）", h.cfg.MaxFiles), nil)
	}

	items := make([]service.BatchItem, 0, len(input.Paths))
	seen := make(map[string]struct{}, len(input.Paths))
	for _, rawPath := range input.Paths {
		path, err := normalizeLocalInputPath(rawPath)
		if err != nil {
			return nil, NewAppError(ErrInputPathDenied, fmt.Sprintf("路径不可用: %s", strings.TrimSpace(rawPath)), err)
		}

		key := strings.ToLower(path)
		if _, exists := seen[key]; exists {
			continue
		}

		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, NewAppError(ErrInputPathDenied, fmt.Sprintf("文件不存在或不可读取: %s", path), err)
		}
		if !containsInputExt(info.Name()) {
			return nil, NewAppError(ErrUnsupportedFormat, fmt.Sprintf("不支持的格式: %s", filepath.Ext(info.Name())), nil)
		}
		if info.Size() > h.cfg.MaxFileSize {
			return nil, NewAppError(
				ErrFileTooLarge,
				fmt.Sprintf("文件 %s 超过大小限制（上限 %d MiB）", info.Name(), h.cfg.MaxFileSize/(1024*1024)),
				nil,
			)
		}

		seen[key] = struct{}{}
		items = append(items, service.BatchItem{
			Path:       path,
			OriginPath: path,
			Name:       info.Name(),
			Size:       info.Size(),
			Current:    len(items) + 1,
		})
	}
	if len(items) == 0 {
		return nil, NewAppError(ErrNoFiles, "未选择可转换文件", nil)
	}

	outputDir, err := validateLocalFolderPath(input.OutputDir)
	if err != nil {
		return nil, NewAppError(ErrOutputRequired, "输出目录无效", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, NewAppError(ErrOutputRequired, "无法创建输出目录", err)
	}

	return &convertRequest{
		Items:        items,
		OutputDir:    outputDir,
		DBPath:       strings.TrimSpace(input.DBPath),
		OutputFormat: service.NormalizeOutputFormat(input.OutputFormat),
		MP3Quality:   service.NormalizeMP3Quality(input.MP3Quality),
		Concurrency:  normalizeConcurrency(input.Concurrency, h.cfg.Concurrency),
		Cleanup:      func() {},
	}, nil
}
