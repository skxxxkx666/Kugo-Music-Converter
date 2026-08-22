package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kugo-music-converter/internal/config"
	"kugo-music-converter/internal/handler"
	"kugo-music-converter/internal/runtimebundle"
	"kugo-music-converter/internal/service"
)

const maxDesktopFiles = 500

var supportedDesktopExts = map[string]struct{}{
	".kgg": {}, ".kgm": {}, ".kgma": {}, ".vpr": {}, ".ncm": {}, ".kwm": {}, ".mflac": {}, ".mgg": {},
	".qmc0": {}, ".qmc2": {}, ".qmc3": {}, ".qmc4": {}, ".qmc6": {}, ".qmc8": {},
	".qmcflac": {}, ".qmcogg": {}, ".tkm": {},
}

var supportedDesktopExtList = []string{
	".kgg", ".kgm", ".kgma", ".vpr", ".ncm", ".kwm", ".mflac", ".mgg",
	".qmc0", ".qmc2", ".qmc3", ".qmc4", ".qmc6", ".qmc8", ".qmcflac", ".qmcogg", ".tkm",
}

const supportedDesktopFilePattern = "*.kgg;*.kgm;*.kgma;*.vpr;*.ncm;*.kwm;*.mflac;*.mgg;*.qmc0;*.qmc2;*.qmc3;*.qmc4;*.qmc6;*.qmc8;*.qmcflac;*.qmcogg;*.tkm"

type StartupState struct {
	Version            string   `json:"version"`
	RuntimeReady       bool     `json:"runtimeReady"`
	RuntimeMessage     string   `json:"runtimeMessage"`
	FFmpegPath         string   `json:"ffmpegPath,omitempty"`
	DefaultOutputDir   string   `json:"defaultOutputDir"`
	DefaultConcurrency int      `json:"defaultConcurrency"`
	MaxConcurrency     int      `json:"maxConcurrency"`
	MaxFileCount       int      `json:"maxFileCount"`
	SupportedExts      []string `json:"supportedExts"`
	DBFound            bool     `json:"dbFound"`
	DBPath             string   `json:"dbPath,omitempty"`
	DBSource           string   `json:"dbSource,omitempty"`
}

type SelectedFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type App struct {
	ctx     context.Context
	version string

	mu            sync.RWMutex
	state         StartupState
	converter     desktopConverter
	activeTaskID  string
	activeCancel  context.CancelFunc
	taskSequence  uint64
	eventSink     func(name string, payload any)
	preview       *desktopPreviewRegistry
	nativeHwnd    uintptr
	nativePercent int
}

func NewApp(version string) *App {
	cleanVersion := strings.TrimSpace(version)
	if cleanVersion == "" {
		cleanVersion = "dev"
	}

	return &App{
		version: cleanVersion,
		preview: newDesktopPreviewRegistry(),
		state: StartupState{
			Version:            cleanVersion,
			RuntimeMessage:     "正在检查 FFmpeg 运行时…",
			DefaultOutputDir:   defaultOutputDirectory(),
			DefaultConcurrency: config.DefaultConfig().Concurrency,
			MaxConcurrency:     config.RuntimeMaxConcurrency(),
			MaxFileCount:       maxDesktopFiles,
			SupportedExts:      append([]string(nil), supportedDesktopExtList...),
		},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.initNativeIntegration()
	a.prepareRuntime()
}

// beforeClose 在转换任务进行中拦截窗口关闭，征得用户确认后才允许退出。
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	a.mu.RLock()
	busy := a.activeTaskID != ""
	a.mu.RUnlock()
	if !busy {
		return false
	}
	choice, err := wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "退出确认",
		Message:       "确定要退出吗？退出会中断未完成的文件；已完成的文件会保留在输出目录。",
		Buttons:       []string{"继续转换", "退出"},
		DefaultButton: "No",
		CancelButton:  "No",
	})
	if err != nil {
		// 对话框不可用时保守处理：阻止退出
		return true
	}
	return !messageDialogConfirmed(choice, "退出")
}

func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activeCancel != nil {
		a.activeCancel()
	}
	a.activeTaskID = ""
	a.activeCancel = nil
}

func (a *App) GetStartupState() StartupState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

func (a *App) SelectAudioFiles() ([]SelectedFile, error) {
	if a.ctx == nil {
		return nil, errors.New("桌面窗口尚未就绪")
	}

	paths, err := wailsruntime.OpenMultipleFilesDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择加密音频文件",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "支持的音频文件", Pattern: supportedDesktopFilePattern},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("打开文件选择器失败: %w", err)
	}
	if len(paths) > maxDesktopFiles {
		return nil, fmt.Errorf("一次最多选择 %d 个文件", maxDesktopFiles)
	}

	files := make([]SelectedFile, 0, len(paths))
	for _, path := range paths {
		cleanPath := filepath.Clean(path)
		if !filepath.IsAbs(cleanPath) || strings.HasPrefix(cleanPath, `\\`) || strings.HasPrefix(cleanPath, `//`) {
			return nil, fmt.Errorf("仅支持本地磁盘中的绝对路径: %s", path)
		}
		if _, ok := supportedDesktopExts[strings.ToLower(filepath.Ext(cleanPath))]; !ok {
			continue
		}
		info, statErr := os.Stat(cleanPath)
		if statErr != nil || info.IsDir() {
			continue
		}
		files = append(files, SelectedFile{Path: cleanPath, Name: info.Name(), Size: info.Size()})
	}
	return files, nil
}

func (a *App) SelectOutputDirectory() (string, error) {
	if a.ctx == nil {
		return "", errors.New("桌面窗口尚未就绪")
	}

	current := a.GetStartupState().DefaultOutputDir
	if info, err := os.Stat(current); err != nil || !info.IsDir() {
		current = ""
	}
	directory, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:                "选择输出目录",
		DefaultDirectory:     current,
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", fmt.Errorf("打开目录选择器失败: %w", err)
	}
	if strings.TrimSpace(directory) == "" {
		return "", nil
	}

	cleanDirectory := filepath.Clean(directory)
	a.mu.Lock()
	a.state.DefaultOutputDir = cleanDirectory
	a.mu.Unlock()
	return cleanDirectory, nil
}

func (a *App) SelectDatabaseFile() (string, error) {
	if a.ctx == nil {
		return "", errors.New("桌面窗口尚未就绪")
	}

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择 KGMusicV3.db",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "酷狗音乐数据库", Pattern: "KGMusicV3.db"},
			{DisplayName: "SQLite 数据库", Pattern: "*.db"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("打开数据库选择器失败: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return "", nil
	}

	return a.useDatabaseFile(path)
}

func (a *App) RestoreDatabaseFile(path string) (string, error) {
	return a.useDatabaseFile(path)
}

func (a *App) RedetectDatabase() (service.DBStatus, error) {
	a.mu.RLock()
	converter := a.converter
	a.mu.RUnlock()
	if converter == nil {
		return service.DBStatus{}, errors.New("转换服务尚未初始化")
	}

	status := converter.RedetectDatabase()
	a.mu.Lock()
	a.state.DBFound = status.Found
	a.state.DBPath = status.Path
	a.state.DBSource = status.Source
	a.mu.Unlock()
	return status, nil
}

func (a *App) useDatabaseFile(path string) (string, error) {
	validation := service.ValidateDBPath(path)
	if !validation.Valid {
		return "", fmt.Errorf("数据库文件无效（%s）", validation.Reason)
	}
	a.mu.Lock()
	a.state.DBFound = true
	a.state.DBPath = validation.Path
	a.state.DBSource = "manual"
	a.mu.Unlock()
	return validation.Path, nil
}

func (a *App) OpenOutputDirectory(directory string) error {
	if goruntime.GOOS != "windows" {
		return errors.New("当前仅支持 Windows 资源管理器")
	}
	trimmed := strings.TrimSpace(directory)
	if trimmed == "" {
		trimmed = a.GetStartupState().DefaultOutputDir
	}
	if strings.TrimSpace(trimmed) == "" {
		return errors.New("输出目录不能为空")
	}
	if strings.HasPrefix(trimmed, `\\`) || strings.HasPrefix(trimmed, `//`) {
		return errors.New("不支持网络共享路径")
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return fmt.Errorf("输出目录无效: %w", err)
	}
	if err := os.MkdirAll(absPath, 0o755); err != nil {
		return fmt.Errorf("无法创建输出目录: %w", err)
	}

	cmd := exec.Command("explorer.exe", absPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("无法打开输出目录: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func (a *App) prepareRuntime() {
	payload, checksum := runtimebundle.FFmpegPayload()
	cacheDirectory, err := os.UserCacheDir()
	if err != nil {
		a.setRuntimeState(false, "无法定位用户缓存目录", "")
		return
	}

	result, err := runtimebundle.EnsureFFmpeg(
		filepath.Join(cacheDirectory, "Kugo Music Converter"),
		payload,
		checksum,
	)
	if errors.Is(err, runtimebundle.ErrPayloadUnavailable) {
		a.setRuntimeState(false, "开发构建未嵌入 FFmpeg；正式构建会在首次启动时自动解压。", "")
		return
	}
	if err != nil {
		a.setRuntimeState(false, fmt.Sprintf("FFmpeg 运行时准备失败：%v", err), "")
		return
	}

	message := "FFmpeg 运行时已就绪"
	if result.Extracted {
		message = "FFmpeg 已解压到用户缓存，后续启动将直接复用"
	}

	cfg := config.DefaultConfig()
	cfg.FFmpegBin = result.Path
	cfg.DefaultOutput = a.GetStartupState().DefaultOutputDir
	converter := handler.NewDesktopConvertHandler(cfg, a.version)
	dbStatus := converter.DatabaseStatus()

	a.mu.Lock()
	a.converter = converter
	a.state.RuntimeReady = true
	a.state.RuntimeMessage = message
	a.state.FFmpegPath = result.Path
	a.state.DefaultConcurrency = cfg.Concurrency
	a.state.MaxConcurrency = config.RuntimeMaxConcurrency()
	a.state.DBFound = dbStatus.Found
	a.state.DBPath = dbStatus.Path
	a.state.DBSource = dbStatus.Source
	a.mu.Unlock()
}

func (a *App) setRuntimeState(ready bool, message string, path string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.RuntimeReady = ready
	a.state.RuntimeMessage = message
	a.state.FFmpegPath = path
}

func defaultOutputDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, "Music", "Kugo Music Converter")
}
