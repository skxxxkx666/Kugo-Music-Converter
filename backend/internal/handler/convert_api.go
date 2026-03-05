package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"kugo-music-converter/internal/logger"
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

	writer := bufio.NewWriterSize(f, 256*1024)
	buf := make([]byte, 128*1024)
	n, err := io.CopyBuffer(writer, src, buf)
	if err != nil {
		return n, err
	}
	if err := writer.Flush(); err != nil {
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

func normalizeLocalInputPath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("路径不能为空")
	}
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(trimmed, `\\`) || strings.HasPrefix(trimmed, `//`) {
			return "", fmt.Errorf("不支持网络共享路径")
		}
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("必须使用绝对路径")
	}

	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("路径无效")
	}

	if runtime.GOOS == "windows" {
		volume := filepath.VolumeName(abs)
		if len(volume) != 2 || volume[1] != ':' {
			return "", fmt.Errorf("仅允许本地磁盘路径")
		}
	}

	return filepath.Clean(abs), nil
}

func uniqueOutputPath(path string) (string, error) {
	// 使用 O_CREATE|O_EXCL 原子创建文件，避免并发 TOCTOU 竞争
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644); err == nil {
		_ = f.Close()
		return path, nil
	} else if !errors.Is(err, os.ErrExist) {
		return path, nil // 目录不存在等其他错误，让后续流程处理
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; i < 10000; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if f, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644); err == nil {
			_ = f.Close()
			return candidate, nil
		} else if !errors.Is(err, os.ErrExist) {
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
		abs, normErr := normalizeLocalInputPath(p)
		if normErr != nil {
			return nil, NewAppError(ErrInputPathDenied, fmt.Sprintf("路径不可用: %s", strings.TrimSpace(p)), normErr)
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
		rawStream   io.ReadCloser
		decryptErr  error
		rawAudioExt string
		audioReader io.Reader
	)

	if ext == ".kgg" {
		if len(dbKeys) == 0 {
			return "", NewAppError(ErrDBNotFound, "KGG 转换需要 KGMusicV3.db", nil)
		}
		rawStream, decryptErr = h.decryptService.DecryptFileByExtWithMemKey(item.Path, dbKeys)
	} else {
		rawStream, decryptErr = h.decryptService.DecryptFileByExt(item.Path)
	}
	if decryptErr != nil {
		return "", NewAppError(detectErrorCode(decryptErr), decryptErr.Error(), decryptErr)
	}
	if rawStream == nil {
		return "", NewAppError(ErrDecryptFailed, "解密返回空数据流", nil)
	}
	defer rawStream.Close()

	if progress != nil {
		progress("decrypt", 60)
	}

	rawAudioExt, audioReader, decryptErr = service.DetectAudioExtFromReader(rawStream)
	if decryptErr != nil {
		logger.Warnf("音频格式识别失败: file=%s err=%v", item.Name, decryptErr)
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
		if err := service.CopyReaderToFile(audioReader, outputPath); err != nil {
			removeQuiet(outputPath)
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
		if err := service.CopyReaderToFile(audioReader, outputPath); err != nil {
			removeQuiet(outputPath)
			return "", NewAppError(ErrTranscodeFailed, "写入输出文件失败", err)
		}
	} else {
		if service.AudioExtToFFmpegFormat(rawAudioExt) == "" {
			removeQuiet(outputPath)
			return "", NewAppError(ErrDecryptFailed, "解密后的音频格式暂不支持转码", nil)
		}

		tmpAudioPath, tmpErr := createTempFile("kgg-raw-audio-", rawAudioExt)
		if tmpErr != nil {
			removeQuiet(outputPath)
			return "", NewAppError("ERR_UNKNOWN", "创建临时音频文件失败", tmpErr)
		}
		defer removeQuiet(tmpAudioPath)

		if err := service.CopyReaderToFile(audioReader, tmpAudioPath); err != nil {
			removeQuiet(outputPath)
			return "", NewAppError(ErrTranscodeFailed, "缓存解密音频失败", err)
		}

		usedTolerance, transErr := service.TranscodeToFormatWithRetry(ctx, h.ffmpegPath, tmpAudioPath, outputPath, req.OutputFormat, req.MP3Quality)
		if transErr != nil {
			removeQuiet(outputPath)

			if usedTolerance && strings.EqualFold(rawAudioExt, ".ogg") {
				fallbackPath, pathErr := uniqueOutputPath(filepath.Join(req.OutputDir, baseName+rawAudioExt))
				if pathErr != nil {
					return "", pathErr
				}
				copyErr := service.CopyFile(tmpAudioPath, fallbackPath)
				if copyErr == nil {
					logger.Warnf("OGG CRC 容错转码失败，已降级为原始 OGG 输出: file=%s output=%s", item.Name, fallbackPath)
					if progress != nil {
						progress("transcode", 100)
					}
					return fallbackPath, nil
				}
				removeQuiet(fallbackPath)
				logger.Warnf("OGG copy 保底输出失败: file=%s err=%v", item.Name, copyErr)
			}

			if ctx.Err() != nil {
				return "", NewAppError(ErrCancelled, "任务已取消", ctx.Err())
			}
			code := h.classifyTranscodeFailure(transErr.Error())
			return "", NewAppError(code, transErr.Error(), transErr)
		}

		if usedTolerance {
			logger.Warnf("触发 OGG CRC 容错重试并成功: file=%s", item.Name)
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

	if h.isShuttingDown() {
		writeError(w, http.StatusServiceUnavailable, NewAppError(ErrRuntimeMissing, "服务正在关闭，请稍后重试", nil))
		return
	}

	summary := h.executeBatch(r.Context(), req, func() bool { return false }, nil)
	h.registerPreviewFiles(summary.Results)
	writeJSON(w, http.StatusOK, summary)
}
