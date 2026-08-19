package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kugo-music-converter/internal/handler"
	"kugo-music-converter/internal/service"
)

const (
	desktopScanTimeout          = 30 * time.Second
	desktopAdvancedScanMaxFiles = 50000
	desktopExportMaxBytes       = 8 << 20
	desktopPreviewRetention     = 24 * time.Hour
	desktopPreviewPrefix        = "/desktop-preview/"
)

var desktopPreviewExts = map[string]struct{}{
	".aac":  {},
	".flac": {},
	".m4a":  {},
	".mp3":  {},
	".ogg":  {},
	".opus": {},
	".wav":  {},
	".wma":  {},
}

type ScanDirectoryRequest struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

type ScanDirectoryResult struct {
	Directory string         `json:"directory"`
	Files     []SelectedFile `json:"files"`
	TotalSize int64          `json:"totalSize"`
	Truncated bool           `json:"truncated"`
	Warning   string         `json:"warning,omitempty"`
}

type ScanDirectoriesRequest struct {
	Paths     []string `json:"paths"`
	Recursive bool     `json:"recursive"`
	Filter    string   `json:"filter"`
}

type DroppedPathsRequest struct {
	Paths     []string `json:"paths"`
	Recursive bool     `json:"recursive"`
}

type DroppedPathsResult struct {
	Files     []SelectedFile `json:"files"`
	Warnings  []string       `json:"warnings,omitempty"`
	Truncated bool           `json:"truncated"`
}

type SaveTextFileRequest struct {
	DefaultFilename string `json:"defaultFilename"`
	Content         string `json:"content"`
	Extension       string `json:"extension"`
}

type desktopPreviewEntry struct {
	path         string
	registeredAt time.Time
}

type desktopPreviewRegistry struct {
	mu      sync.Mutex
	byPath  map[string]string
	byToken map[string]desktopPreviewEntry
}

func newDesktopPreviewRegistry() *desktopPreviewRegistry {
	return &desktopPreviewRegistry{
		byPath:  make(map[string]string),
		byToken: make(map[string]desktopPreviewEntry),
	}
}

func (a *App) SelectScanDirectory() (string, error) {
	if a.ctx == nil {
		return "", errors.New("桌面窗口尚未就绪")
	}

	directory, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择要扫描的音乐目录",
	})
	if err != nil {
		return "", fmt.Errorf("打开目录选择器失败: %w", err)
	}
	if strings.TrimSpace(directory) == "" {
		return "", nil
	}
	return validateDesktopDirectory(directory)
}

func (a *App) ScanAudioDirectory(request ScanDirectoryRequest) (ScanDirectoryResult, error) {
	directory, err := validateDesktopDirectory(request.Path)
	if err != nil {
		return ScanDirectoryResult{}, err
	}

	parentCtx := a.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, desktopScanTimeout)
	defer cancel()

	files, totalSize, scanErr := service.ScanSingleFolderCtx(
		ctx,
		directory,
		request.Recursive,
		supportedDesktopExts,
		maxDesktopFiles,
	)
	result := ScanDirectoryResult{
		Directory: directory,
		Files:     make([]SelectedFile, 0, len(files)),
		TotalSize: totalSize,
	}
	for _, file := range files {
		result.Files = append(result.Files, SelectedFile{
			Path: file.FullPath,
			Name: file.Name,
			Size: file.Size,
		})
	}

	switch {
	case errors.Is(scanErr, service.ErrScanLimitReached):
		result.Truncated = true
		result.Warning = fmt.Sprintf("扫描结果已达到 %d 个文件上限，请缩小目录范围。", maxDesktopFiles)
	case scanErr != nil:
		return ScanDirectoryResult{}, fmt.Errorf("扫描目录失败: %w", scanErr)
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		result.Truncated = true
		result.Warning = "扫描已超时，仅保留当前找到的文件。"
	case errors.Is(ctx.Err(), context.Canceled):
		return ScanDirectoryResult{}, errors.New("扫描已取消")
	}

	return result, nil
}

func (a *App) ScanDirectories(request ScanDirectoriesRequest) (service.ScanResult, error) {
	if len(request.Paths) == 0 {
		return service.ScanResult{}, errors.New("请先选择至少一个扫描目录")
	}
	if len(request.Paths) > 32 {
		return service.ScanResult{}, errors.New("一次最多扫描 32 个目录")
	}

	parentCtx := a.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, desktopScanTimeout)
	defer cancel()

	result := service.ScanResult{Folders: make([]service.ScanFolderInfo, 0, len(request.Paths))}
	filter := service.ParseExtFilter(request.Filter)
	remaining := desktopAdvancedScanMaxFiles
	for _, rawPath := range request.Paths {
		directory, err := validateDesktopDirectory(rawPath)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("目录不可访问：%s", strings.TrimSpace(rawPath)))
			continue
		}

		files, size, scanErr := service.ScanSingleFolderCtx(ctx, directory, request.Recursive, filter, remaining)
		if len(files) > 0 {
			result.Folders = append(result.Folders, service.ScanFolderInfo{Path: directory, Files: files})
			result.TotalFiles += len(files)
			result.TotalSize += size
			remaining -= len(files)
		}
		if errors.Is(scanErr, service.ErrScanLimitReached) || remaining <= 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("扫描达到上限 %d 个文件，结果已截断。", desktopAdvancedScanMaxFiles))
			break
		}
		if scanErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("扫描失败：%s", directory))
			continue
		}
		if ctx.Err() != nil {
			result.Warnings = append(result.Warnings, "扫描已提前终止（取消或超时）。")
			break
		}
	}
	return result, nil
}

func (a *App) ResolveDroppedPaths(request DroppedPathsRequest) (DroppedPathsResult, error) {
	if len(request.Paths) == 0 {
		return DroppedPathsResult{}, nil
	}
	if len(request.Paths) > maxDesktopFiles {
		return DroppedPathsResult{}, fmt.Errorf("一次最多拖入 %d 个项目", maxDesktopFiles)
	}

	parentCtx := a.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, desktopScanTimeout)
	defer cancel()

	result := DroppedPathsResult{Files: make([]SelectedFile, 0, len(request.Paths))}
	seen := make(map[string]struct{})
	appendFile := func(path string, name string, size int64) bool {
		key := normalizeDesktopPath(path)
		if _, exists := seen[key]; exists {
			return true
		}
		if len(result.Files) >= maxDesktopFiles {
			result.Truncated = true
			return false
		}
		seen[key] = struct{}{}
		result.Files = append(result.Files, SelectedFile{Path: path, Name: name, Size: size})
		return true
	}

	for _, rawPath := range request.Paths {
		trimmed := strings.TrimSpace(rawPath)
		if trimmed == "" || strings.HasPrefix(trimmed, `\\`) || strings.HasPrefix(trimmed, `//`) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("路径不可用：%s", trimmed))
			continue
		}
		absPath, err := filepath.Abs(trimmed)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("路径无效：%s", trimmed))
			continue
		}
		info, err := os.Stat(absPath)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("路径不存在：%s", absPath))
			continue
		}
		if info.IsDir() {
			remaining := maxDesktopFiles - len(result.Files)
			files, _, scanErr := service.ScanSingleFolderCtx(ctx, absPath, request.Recursive, supportedDesktopExts, remaining)
			for _, file := range files {
				if !appendFile(file.FullPath, file.Name, file.Size) {
					break
				}
			}
			if errors.Is(scanErr, service.ErrScanLimitReached) {
				result.Truncated = true
			}
			if scanErr != nil && !errors.Is(scanErr, service.ErrScanLimitReached) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("目录扫描失败：%s", absPath))
			}
		} else if info.Mode().IsRegular() {
			ext := strings.ToLower(filepath.Ext(info.Name()))
			if _, ok := supportedDesktopExts[ext]; !ok {
				result.Warnings = append(result.Warnings, fmt.Sprintf("已跳过不支持的文件：%s", info.Name()))
				continue
			}
			if !appendFile(filepath.Clean(absPath), info.Name(), info.Size()) {
				break
			}
		}
		if result.Truncated || ctx.Err() != nil {
			break
		}
	}
	if result.Truncated {
		result.Warnings = append(result.Warnings, fmt.Sprintf("拖入结果已达到 %d 个文件上限。", maxDesktopFiles))
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.Warnings = append(result.Warnings, "文件夹扫描已超时，仅保留当前结果。")
	}
	return result, nil
}

func (a *App) SaveTextFile(request SaveTextFileRequest) (string, error) {
	if a.ctx == nil {
		return "", errors.New("桌面窗口尚未就绪")
	}
	filename, extension, err := normalizeTextExportRequest(request)
	if err != nil {
		return "", err
	}
	displayName := "文本文件"
	if extension == ".csv" {
		displayName = "CSV 文件"
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "保存导出文件",
		DefaultFilename: filename,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: displayName, Pattern: "*" + extension},
		},
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", fmt.Errorf("打开保存对话框失败: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`) {
		return "", errors.New("不支持保存到网络共享路径")
	}
	if !strings.EqualFold(filepath.Ext(path), extension) {
		path += extension
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("保存路径无效: %w", err)
	}
	if err := os.WriteFile(absPath, []byte(request.Content), 0o644); err != nil {
		return "", fmt.Errorf("保存导出文件失败: %w", err)
	}
	return filepath.Clean(absPath), nil
}

func (a *App) ConfirmHistoryAction(action string) (bool, error) {
	if a.ctx == nil {
		return false, errors.New("桌面窗口尚未就绪")
	}
	title := "删除历史记录"
	message := "确定删除这条历史记录吗？"
	if action == "clear" {
		title = "清空历史记录"
		message = "确定清空全部转换历史吗？此操作无法撤销。"
	} else if action != "delete" {
		return false, errors.New("不支持的确认操作")
	}
	result, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         title,
		Message:       message,
		Buttons:       []string{"删除", "取消"},
		DefaultButton: "No",
		CancelButton:  "No",
	})
	if err != nil {
		return false, fmt.Errorf("打开确认窗口失败: %w", err)
	}
	return messageDialogConfirmed(result, "删除"), nil
}

// Windows MessageBox ignores custom labels for question dialogs and returns
// "Yes" or "No". Other Wails backends may return the configured label.
func messageDialogConfirmed(result string, affirmativeLabel string) bool {
	value := strings.TrimSpace(result)
	return strings.EqualFold(value, "yes") || value == affirmativeLabel
}

func (a *App) ConfirmConversionCancellation() (bool, error) {
	if a.ctx == nil {
		return false, errors.New("桌面窗口尚未就绪")
	}
	result, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "取消转换",
		Message:       "确定取消当前转换任务吗？已成功生成的文件会保留。",
		Buttons:       []string{"继续转换", "取消任务"},
		DefaultButton: "No",
		CancelButton:  "No",
	})
	if err != nil {
		return false, fmt.Errorf("打开确认窗口失败: %w", err)
	}
	return messageDialogConfirmed(result, "取消任务"), nil
}

func (a *App) CheckForUpdates() (*handler.ReleaseInfo, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	release, err := handler.CheckLatestRelease(ctx)
	if err != nil {
		return nil, errors.New("无法连接 GitHub，请检查网络连接或代理设置")
	}
	return release, nil
}

func (a *App) OpenReleasePage(rawURL string) error {
	if a.ctx == nil {
		return errors.New("桌面窗口尚未就绪")
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return errors.New("更新链接无效")
	}
	if !strings.HasPrefix(strings.ToLower(parsed.EscapedPath()), "/skxxxkx666/kugo-music-converter/releases") {
		return errors.New("仅允许打开本项目的 GitHub Releases 页面")
	}
	wailsruntime.BrowserOpenURL(a.ctx, parsed.String())
	return nil
}

func (a *App) GetPreviewURL(outputPath string) (string, error) {
	if a.preview == nil {
		return "", errors.New("试听服务尚未初始化")
	}
	realPath, err := resolveDesktopFile(outputPath)
	if err != nil {
		return "", errors.New("试听文件不可用")
	}
	token, ok := a.preview.lookupPath(realPath)
	if !ok {
		return "", errors.New("该文件不在本次运行的转换结果中")
	}
	return desktopPreviewPrefix + token, nil
}

func (a *App) OpenOutputFile(outputPath string) error {
	if a.preview == nil {
		return errors.New("输出文件注册表尚未初始化")
	}
	realPath, err := resolveDesktopFile(outputPath)
	if err != nil {
		return errors.New("输出文件不可用")
	}
	if _, ok := a.preview.lookupPath(realPath); !ok {
		return errors.New("该文件不在本次运行的转换结果中")
	}

	cmd := exec.Command("explorer.exe", "/select,", realPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("无法在资源管理器中定位文件: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func (a *App) registerPreviewResult(result service.BatchFileDoneEvent) {
	if result.Status != "ok" || strings.TrimSpace(result.Output) == "" || a.preview == nil {
		return
	}
	realPath, err := resolveDesktopFile(result.Output)
	if err != nil {
		return
	}
	if _, ok := desktopPreviewExts[strings.ToLower(filepath.Ext(realPath))]; !ok {
		return
	}
	_ = a.preview.register(realPath)
}

func (a *App) assetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, desktopPreviewPrefix) {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		token := strings.TrimPrefix(r.URL.Path, desktopPreviewPrefix)
		if token == "" || strings.Contains(token, "/") || a.preview == nil {
			http.NotFound(w, r)
			return
		}
		entry, ok := a.preview.lookupToken(token)
		if !ok {
			http.NotFound(w, r)
			return
		}

		file, err := os.Open(entry.path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}

		contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(entry.path)))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store")
		http.ServeContent(w, r, filepath.Base(entry.path), info.ModTime(), file)
	})
}

func (r *desktopPreviewRegistry) register(path string) error {
	if r == nil {
		return errors.New("preview registry is nil")
	}
	tokenBytes := make([]byte, 18)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes)
	now := time.Now()
	key := normalizeDesktopPath(path)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(now)
	if existingToken, ok := r.byPath[key]; ok {
		r.byToken[existingToken] = desktopPreviewEntry{path: path, registeredAt: now}
		return nil
	}
	r.byPath[key] = token
	r.byToken[token] = desktopPreviewEntry{path: path, registeredAt: now}
	return nil
}

func (r *desktopPreviewRegistry) lookupPath(path string) (string, bool) {
	if r == nil {
		return "", false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(time.Now())
	token, ok := r.byPath[normalizeDesktopPath(path)]
	return token, ok
}

func (r *desktopPreviewRegistry) lookupToken(token string) (desktopPreviewEntry, bool) {
	if r == nil {
		return desktopPreviewEntry{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(time.Now())
	entry, ok := r.byToken[token]
	return entry, ok
}

func (r *desktopPreviewRegistry) pruneLocked(now time.Time) {
	for token, entry := range r.byToken {
		if now.Sub(entry.registeredAt) <= desktopPreviewRetention {
			continue
		}
		delete(r.byToken, token)
		delete(r.byPath, normalizeDesktopPath(entry.path))
	}
}

func validateDesktopDirectory(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", errors.New("扫描目录不能为空")
	}
	if strings.HasPrefix(trimmed, `\\`) || strings.HasPrefix(trimmed, `//`) {
		return "", errors.New("不支持网络共享目录")
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("扫描目录无效: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return "", errors.New("扫描目录不存在或不可访问")
	}
	return filepath.Clean(absPath), nil
}

func resolveDesktopFile(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" || strings.HasPrefix(trimmed, `\\`) || strings.HasPrefix(trimmed, `//`) {
		return "", errors.New("file path is invalid")
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", err
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(realPath)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("file is unavailable")
	}
	return filepath.Clean(realPath), nil
}

func normalizeDesktopPath(path string) string {
	return strings.ToLower(filepath.Clean(path))
}

func normalizeTextExportRequest(request SaveTextFileRequest) (string, string, error) {
	if len([]byte(request.Content)) > desktopExportMaxBytes {
		return "", "", fmt.Errorf("导出内容超过 %d MiB 上限", desktopExportMaxBytes/(1<<20))
	}
	extension := strings.ToLower(strings.TrimSpace(request.Extension))
	if extension != ".csv" && extension != ".txt" && extension != ".log" {
		return "", "", errors.New("仅支持导出 CSV、TXT 或 LOG 文件")
	}

	filename := filepath.Base(strings.TrimSpace(request.DefaultFilename))
	if filename == "" || filename == "." {
		filename = "Kugo-Music-Converter-Export" + extension
	}
	if !strings.EqualFold(filepath.Ext(filename), extension) {
		filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + extension
	}
	return filename, extension, nil
}
