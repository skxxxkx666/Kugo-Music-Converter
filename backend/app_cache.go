package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kugo-music-converter/internal/webview2bundle"
)

type RuntimeCacheInfo struct {
	Root             string `json:"root"`
	RetainedBytes    int64  `json:"retainedBytes"`
	ReclaimableBytes int64  `json:"reclaimableBytes"`
	ReclaimableItems int    `json:"reclaimableItems"`
	FFmpegBytes      int64  `json:"ffmpegBytes"`
	WebView2Bytes    int64  `json:"webview2Bytes"`
	UpdateBytes      int64  `json:"updateBytes"`
}

type RuntimeCacheClearResult struct {
	FreedBytes    int64            `json:"freedBytes"`
	RemovedItems  int              `json:"removedItems"`
	Cancelled     bool             `json:"cancelled"`
	Warning       string           `json:"warning,omitempty"`
	RemainingInfo RuntimeCacheInfo `json:"remainingInfo"`
}

type runtimeCacheEntry struct {
	path      string
	size      int64
	protected bool
}

func (a *App) GetRuntimeCacheInfo() (RuntimeCacheInfo, error) {
	root, err := desktopCacheRoot()
	if err != nil {
		return RuntimeCacheInfo{}, err
	}
	info, _, err := inspectRuntimeCache(root, a.activeRuntimeCacheDirectories(root))
	return info, err
}

func (a *App) ClearRuntimeCache() (RuntimeCacheClearResult, error) {
	if a.ctx == nil {
		return RuntimeCacheClearResult{}, errors.New("桌面窗口尚未就绪")
	}
	root, err := desktopCacheRoot()
	if err != nil {
		return RuntimeCacheClearResult{}, err
	}
	info, entries, err := inspectRuntimeCache(root, a.activeRuntimeCacheDirectories(root))
	if err != nil {
		return RuntimeCacheClearResult{}, err
	}
	if info.ReclaimableItems == 0 {
		return RuntimeCacheClearResult{RemainingInfo: info}, nil
	}

	choice, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:  wailsruntime.QuestionDialog,
		Title: "清理运行时缓存",
		Message: fmt.Sprintf(
			"将清理 %d 项旧版 FFmpeg、旧版内嵌 WebView2 或更新临时文件（约 %s）。\n\n当前运行时、音乐文件、输出文件和转换历史不会被删除。",
			info.ReclaimableItems,
			formatCacheBytes(info.ReclaimableBytes),
		),
		Buttons:       []string{"清理缓存", "取消"},
		DefaultButton: "No",
		CancelButton:  "No",
	})
	if err != nil {
		return RuntimeCacheClearResult{}, fmt.Errorf("打开缓存清理确认窗口失败: %w", err)
	}
	if !messageDialogConfirmed(choice, "清理缓存") {
		return RuntimeCacheClearResult{Cancelled: true, RemainingInfo: info}, nil
	}

	result := clearRuntimeCacheEntries(entries)
	remainingInfo, _, inspectErr := inspectRuntimeCache(root, a.activeRuntimeCacheDirectories(root))
	if inspectErr != nil {
		return RuntimeCacheClearResult{}, inspectErr
	}
	result.RemainingInfo = remainingInfo
	return result, nil
}

func desktopCacheRoot() (string, error) {
	cacheDirectory, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("定位用户缓存目录: %w", err)
	}
	return filepath.Join(cacheDirectory, "Kugo Music Converter"), nil
}

func (a *App) activeRuntimeCacheDirectories(root string) []string {
	active := make([]string, 0, 2)
	a.mu.RLock()
	ffmpegPath := a.state.FFmpegPath
	a.mu.RUnlock()
	if strings.TrimSpace(ffmpegPath) != "" {
		active = append(active, filepath.Dir(ffmpegPath))
	}

	payload := webview2bundle.EmbeddedPayload()
	version := strings.TrimSpace(payload.Version)
	hash := strings.ToLower(strings.TrimSpace(payload.ExpectedSHA256))
	if version != "" && len(hash) >= 16 {
		active = append(active, filepath.Join(root, "webview2", version+"-"+hash[:16]))
	}
	return active
}

func inspectRuntimeCache(root string, activeDirectories []string) (RuntimeCacheInfo, []runtimeCacheEntry, error) {
	cleanRoot := filepath.Clean(root)
	info := RuntimeCacheInfo{Root: cleanRoot}
	protected := make(map[string]struct{}, len(activeDirectories))
	for _, path := range activeDirectories {
		cleanPath := filepath.Clean(path)
		if pathInside(cleanRoot, cleanPath) {
			protected[strings.ToLower(cleanPath)] = struct{}{}
		}
	}

	entries := make([]runtimeCacheEntry, 0)
	for _, category := range []string{"runtime", "webview2", "updates"} {
		categoryRoot := filepath.Join(cleanRoot, category)
		children, err := os.ReadDir(categoryRoot)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return RuntimeCacheInfo{}, nil, fmt.Errorf("读取 %s 缓存失败: %w", category, err)
		}
		for _, child := range children {
			path := filepath.Join(categoryRoot, child.Name())
			size := cacheEntrySize(path)
			_, isProtected := protected[strings.ToLower(filepath.Clean(path))]
			entry := runtimeCacheEntry{path: path, size: size, protected: isProtected}
			entries = append(entries, entry)
			switch category {
			case "runtime":
				info.FFmpegBytes += size
			case "webview2":
				info.WebView2Bytes += size
			case "updates":
				info.UpdateBytes += size
			}
			if isProtected {
				info.RetainedBytes += size
			} else {
				info.ReclaimableBytes += size
				info.ReclaimableItems++
			}
		}
	}
	return info, entries, nil
}

func clearRuntimeCacheEntries(entries []runtimeCacheEntry) RuntimeCacheClearResult {
	result := RuntimeCacheClearResult{}
	warnings := make([]string, 0)
	for _, entry := range entries {
		if entry.protected {
			continue
		}
		if err := os.RemoveAll(entry.path); err != nil {
			warnings = append(warnings, filepath.Base(entry.path))
			continue
		}
		result.FreedBytes += entry.size
		result.RemovedItems++
	}
	if len(warnings) > 0 {
		result.Warning = "以下缓存正在使用或无法删除：" + strings.Join(warnings, "、")
	}
	return result
}

func cacheEntrySize(path string) int64 {
	var size int64
	_ = filepath.WalkDir(path, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if info, err := entry.Info(); err == nil {
				size += info.Size()
			}
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			if info, err := entry.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size
}

func pathInside(root string, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func formatCacheBytes(size int64) string {
	const (
		kiB = int64(1024)
		miB = 1024 * kiB
		giB = 1024 * miB
	)
	switch {
	case size >= giB:
		return fmt.Sprintf("%.2f GiB", float64(size)/float64(giB))
	case size >= miB:
		return fmt.Sprintf("%.1f MiB", float64(size)/float64(miB))
	case size >= kiB:
		return fmt.Sprintf("%.1f KiB", float64(size)/float64(kiB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}
