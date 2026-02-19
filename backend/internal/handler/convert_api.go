package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"kugo-music-converter/internal/service"
)

type convertRequest struct {
	Items        []service.BatchItem
	OutputDir    string
	DBPath       string
	OutputFormat string
	MP3Quality   int
	Concurrency  int
	Cleanup      func()
}

const maxConvertRequestBody int64 = 2 << 30 // 2 GiB hard cap

func createTempFile(prefix, suffix string) (string, error) {
	f, err := os.CreateTemp("", prefix+"*"+suffix)
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	return name, nil
}

func copyStreamToFile(src io.Reader, dst string) (int64, error) {
	f, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n, err := io.Copy(f, src)
	if err != nil {
		return n, err
	}
	return n, f.Sync()
}

func removeQuiet(path string) {
	_ = os.Remove(path)
}

func parseIntOrDefault(raw string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return n
}

func uniqueOutputPath(path string) (string, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path, nil
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; i < 10000; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", NewAppError(ErrTranscodeFailed, "输出文件重名过多，无法生成唯一文件名", nil)
}

func parseInputPathItems(raw string) ([]service.BatchItem, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var paths []string
	if err := json.Unmarshal([]byte(raw), &paths); err != nil {
		return nil, NewAppError(ErrNoFiles, "inputPaths 不是合法 JSON 数组", err)
	}

	items := make([]service.BatchItem, 0, len(paths))
	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			continue
		}
		st, err := os.Stat(abs)
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		name := filepath.Base(abs)
		if !containsInputExt(name) {
			continue
		}
		items = append(items, service.BatchItem{
			Path:       abs,
			OriginPath: abs,
			Name:       name,
			Size:       st.Size(),
			Temporary:  false,
		})
	}
	return items, nil
}

func copyUploadToTemp(file multipart.File, hdr *multipart.FileHeader) (service.BatchItem, error) {
	name := hdr.Filename
	if !containsInputExt(name) {
		return service.BatchItem{}, NewAppError(ErrUnsupportedFormat, fmt.Sprintf("不支持的格式: %s", filepath.Ext(name)), nil)
	}

	tmp, err := createTempFile("kgg-upload-", filepath.Ext(name))
	if err != nil {
		return service.BatchItem{}, NewAppError("ERR_UNKNOWN", "创建临时文件失败", err)
	}

	if _, err := copyStreamToFile(file, tmp); err != nil {
		removeQuiet(tmp)
		return service.BatchItem{}, NewAppError("ERR_UNKNOWN", "写入临时文件失败", err)
	}

	st, err := os.Stat(tmp)
	if err != nil {
		removeQuiet(tmp)
		return service.BatchItem{}, NewAppError("ERR_UNKNOWN", "读取临时文件失败", err)
	}

	return service.BatchItem{
		Path:       tmp,
		OriginPath: hdr.Filename,
		Name:       name,
		Size:       st.Size(),
		Temporary:  true,
	}, nil
}

func (h *ConvertHandler) parseConvertRequest(w http.ResponseWriter, r *http.Request) (*convertRequest, error) {
	maxBody := int64(h.cfg.MaxFiles)*h.cfg.MaxFileSize + (20 << 20)
	if maxBody <= 0 || maxBody > maxConvertRequestBody {
		maxBody = maxConvertRequestBody
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	if err := r.ParseMultipartForm(h.cfg.ParseFormMemory); err != nil {
		return nil, NewAppError(ErrFileTooLarge, "表单解析失败或文件超过限制", err)
	}

	items := make([]service.BatchItem, 0, h.cfg.MaxFiles)
	cleanupPaths := make([]string, 0, h.cfg.MaxFiles)
	cleanup := func() {
		for _, p := range cleanupPaths {
			removeQuiet(p)
		}
	}

	fileGroups := [][]*multipart.FileHeader{
		r.MultipartForm.File["kggFiles"],
		r.MultipartForm.File["files"],
	}

	for _, group := range fileGroups {
		for _, hdr := range group {
			if hdr.Size > h.cfg.MaxFileSize {
				cleanup()
				return nil, NewAppError(ErrFileTooLarge, fmt.Sprintf("文件 %s 超过大小限制", hdr.Filename), nil)
			}
			f, err := hdr.Open()
			if err != nil {
				cleanup()
				return nil, NewAppError("ERR_UNKNOWN", "打开上传文件失败", err)
			}
			item, err := copyUploadToTemp(f, hdr)
			_ = f.Close()
			if err != nil {
				cleanup()
				return nil, err
			}
			items = append(items, item)
			cleanupPaths = append(cleanupPaths, item.Path)
		}
	}

	pathItems, err := parseInputPathItems(r.FormValue("inputPaths"))
	if err != nil {
		cleanup()
		return nil, err
	}
	items = append(items, pathItems...)

	if len(items) == 0 {
		cleanup()
		return nil, NewAppError(ErrNoFiles, "未上传可转换文件", nil)
	}
	if len(items) > h.cfg.MaxFiles {
		cleanup()
		return nil, NewAppError(ErrTooManyFiles, fmt.Sprintf("文件数量超过限制（最多 %d）", h.cfg.MaxFiles), nil)
	}
	for _, item := range items {
		if item.Size > h.cfg.MaxFileSize {
			cleanup()
			return nil, NewAppError(ErrFileTooLarge, fmt.Sprintf("文件 %s 超过大小限制", item.Name), nil)
		}
	}

	outputDir := strings.TrimSpace(r.FormValue("outputDir"))
	if outputDir == "" {
		cleanup()
		return nil, NewAppError(ErrOutputRequired, "输出目录不能为空", nil)
	}
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		cleanup()
		return nil, NewAppError(ErrOutputRequired, "输出目录无效", err)
	}
	if err := os.MkdirAll(absOutputDir, 0o755); err != nil {
		cleanup()
		return nil, NewAppError(ErrOutputRequired, "无法创建输出目录", err)
	}

	outputFormat := service.NormalizeOutputFormat(r.FormValue("outputFormat"))
	mp3Quality := service.NormalizeMP3Quality(parseIntOrDefault(r.FormValue("mp3Quality"), 2))
	concurrency := normalizeConcurrency(parseIntOrDefault(r.FormValue("concurrency"), h.cfg.Concurrency), h.cfg.Concurrency)
	dbPath := strings.TrimSpace(r.FormValue("dbPath"))

	for i := range items {
		items[i].Current = i + 1
	}

	return &convertRequest{
		Items:        items,
		OutputDir:    absOutputDir,
		DBPath:       dbPath,
		OutputFormat: outputFormat,
		MP3Quality:   mp3Quality,
		Concurrency:  concurrency,
		Cleanup:      cleanup,
	}, nil
}

func hasKGG(items []service.BatchItem) bool {
	for _, item := range items {
		if strings.EqualFold(filepath.Ext(item.Name), ".kgg") {
			return true
		}
	}
	return false
}

func (h *ConvertHandler) convertSingleItem(ctx context.Context, item service.BatchItem, req *convertRequest, dbKeys map[string]string, progress func(string, int)) (string, error) {
	if progress != nil {
		progress("prepare", 5)
	}
	if ctx.Err() != nil {
		return "", NewAppError(ErrCancelled, "任务已取消", ctx.Err())
	}

	ext := strings.ToLower(filepath.Ext(item.Name))
	var (
		rawPath     string
		rawCleanup  func()
		decryptErr  error
		rawAudioExt string
	)

	if ext == ".kgg" {
		if len(dbKeys) == 0 {
			return "", NewAppError(ErrDBNotFound, "KGG 转换需要 KGMusicV3.db", nil)
		}
		rawPath, rawCleanup, decryptErr = h.decryptService.DecryptFileByExtWithMemKey(item.Path, dbKeys)
	} else {
		rawPath, rawCleanup, decryptErr = h.decryptService.DecryptFileByExt(item.Path)
	}
	if decryptErr != nil {
		return "", NewAppError(detectErrorCode(decryptErr), decryptErr.Error(), decryptErr)
	}
	if rawCleanup != nil {
		defer rawCleanup()
	}

	if progress != nil {
		progress("decrypt", 60)
	}

	rawAudioExt, decryptErr = service.DetectAudioExt(rawPath)
	if decryptErr != nil {
		return "", NewAppError(ErrDecryptFailed, "无法识别解密后的音频格式", decryptErr)
	}

	baseName := strings.TrimSuffix(item.Name, filepath.Ext(item.Name))

	if req.OutputFormat == "copy" {
		outputPath, err := uniqueOutputPath(filepath.Join(req.OutputDir, baseName+rawAudioExt))
		if err != nil {
			return "", err
		}
		if progress != nil {
			progress("transcode", 80)
		}
		if err := service.CopyFile(rawPath, outputPath); err != nil {
			return "", NewAppError(ErrTranscodeFailed, "写入输出文件失败", err)
		}
		if progress != nil {
			progress("transcode", 100)
		}
		return outputPath, nil
	}

	outputPath, err := uniqueOutputPath(service.BuildOutputPath(req.OutputDir, baseName, req.OutputFormat))
	if err != nil {
		return "", err
	}

	if progress != nil {
		progress("transcode", 80)
	}

	targetExt := "." + req.OutputFormat
	if strings.EqualFold(rawAudioExt, targetExt) {
		if err := service.CopyFile(rawPath, outputPath); err != nil {
			return "", NewAppError(ErrTranscodeFailed, "写入输出文件失败", err)
		}
	} else {
		if err := service.TranscodeToFormat(ctx, h.ffmpegPath, rawPath, outputPath, req.OutputFormat, req.MP3Quality); err != nil {
			if ctx.Err() != nil {
				return "", NewAppError(ErrCancelled, "任务已取消", ctx.Err())
			}
			return "", NewAppError(ErrTranscodeFailed, err.Error(), err)
		}
	}

	if progress != nil {
		progress("transcode", 100)
	}
	return outputPath, nil
}

func (h *ConvertHandler) executeBatch(ctx context.Context, req *convertRequest, stopFn func() bool, onEvent func(string, any)) service.BatchSummary {
	runCtx, cancel := h.contextWithShutdown(ctx)
	defer cancel()

	var dbKeys map[string]string
	if hasKGG(req.Items) {
		_, _, keys, err := h.getDBForRequest(req.DBPath)
		if err != nil {
			results := make([]service.BatchFileDoneEvent, 0, len(req.Items))
			for _, item := range req.Items {
				results = append(results, service.BatchFileDoneEvent{
					File:    item.Name,
					Input:   item.OriginPath,
					Status:  "error",
					Error:   toBatchFileError(err),
					Current: item.Current,
					Total:   len(req.Items),
					Percent: 0,
				})
			}
			return service.BatchSummary{
				Success:      0,
				Failed:       len(req.Items),
				Total:        len(req.Items),
				OutputDir:    req.OutputDir,
				OutputFormat: req.OutputFormat,
				MP3Quality:   req.MP3Quality,
				Cancelled:    false,
				Results:      results,
			}
		}
		dbKeys = keys
	}

	var eventMu sync.Mutex
	send := func(name string, payload any) {
		if onEvent == nil {
			return
		}
		eventMu.Lock()
		onEvent(name, payload)
		eventMu.Unlock()
	}

	shouldStop := func() bool {
		if h.isShuttingDown() {
			return true
		}
		if stopFn != nil && stopFn() {
			return true
		}
		return false
	}

	summary := service.RunBatch(runCtx, service.BatchOptions{
		Items:        req.Items,
		Concurrency:  req.Concurrency,
		OutputDir:    req.OutputDir,
		OutputFormat: req.OutputFormat,
		MP3Quality:   req.MP3Quality,
		ShouldStop:   shouldStop,
		ErrorMapper:  toBatchFileError,
		Convert: func(ctx context.Context, item service.BatchItem, progress func(phase string, filePercent int)) (string, error) {
			defer func() {
				if item.Temporary {
					removeQuiet(item.Path)
				}
			}()
			return h.convertSingleItem(ctx, item, req, dbKeys, progress)
		},
		OnProgress: func(event service.BatchProgressEvent) {
			send("progress", event)
		},
		OnFileDone: func(event service.BatchFileDoneEvent) {
			send("file-done", event)
		},
	})

	return summary
}

func (h *ConvertHandler) HandleConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	if missing := h.runtimeMissingTools(); len(missing) > 0 {
		writeError(w, http.StatusServiceUnavailable, NewAppError(ErrRuntimeMissing, strings.Join(missing, ","), nil))
		return
	}

	req, err := h.parseConvertRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer req.Cleanup()

	summary := h.executeBatch(r.Context(), req, func() bool { return false }, nil)
	writeJSON(w, http.StatusOK, summary)
}
