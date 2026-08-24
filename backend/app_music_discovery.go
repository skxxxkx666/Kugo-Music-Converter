package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kugo-music-converter/internal/service"
)

type LocalMusicGroup struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Icon    string         `json:"icon,omitempty"`
	IconImg string         `json:"iconImg,omitempty"`
	Files   []SelectedFile `json:"files"`
}

type FindLocalMusicResult struct {
	Groups    []LocalMusicGroup `json:"groups"`
	Warnings  []string          `json:"warnings,omitempty"`
	Truncated bool              `json:"truncated"`
}

type musicSearchDirectory struct {
	Path      string
	Recursive bool
}

type musicSearchSource struct {
	ID          string
	Name        string
	Icon        string
	IconImg     string
	Directories []musicSearchDirectory
}

// FindLocalMusic searches only client-configured and well-known download folders.
// It never scans an entire drive and never modifies the discovered files.
func (a *App) FindLocalMusic() (FindLocalMusicResult, error) {
	parentCtx := a.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, desktopScanTimeout)
	defer cancel()

	return findLocalMusic(ctx, defaultMusicSearchSources(), maxDesktopFiles)
}

func findLocalMusic(ctx context.Context, sources []musicSearchSource, maxFiles int) (FindLocalMusicResult, error) {
	if maxFiles <= 0 {
		return FindLocalMusicResult{}, errors.New("本机音乐扫描文件上限无效")
	}

	result := FindLocalMusicResult{Groups: make([]LocalMusicGroup, 0, len(sources))}
	groups := make(map[string]*LocalMusicGroup, len(sources))
	groupOrder := make([]string, 0, len(sources))
	for _, source := range sources {
		if _, exists := groups[source.ID]; exists {
			continue
		}
		groups[source.ID] = &LocalMusicGroup{
			ID:      source.ID,
			Name:    source.Name,
			Icon:    source.Icon,
			IconImg: source.IconImg,
			Files:   make([]SelectedFile, 0),
		}
		groupOrder = append(groupOrder, source.ID)
	}
	seenFiles := make(map[string]struct{})
	remaining := maxFiles

	for _, source := range sources {
		if ctx.Err() != nil || remaining <= 0 {
			result.Truncated = true
			break
		}

		group := groups[source.ID]
		for _, directory := range normalizeMusicSearchDirectories(source.Directories) {
			if ctx.Err() != nil || remaining <= 0 {
				result.Truncated = true
				break
			}
			info, err := os.Stat(directory.Path)
			if err != nil || !info.IsDir() {
				continue
			}

			files, _, scanErr := service.ScanSingleFolderCtx(
				ctx,
				directory.Path,
				directory.Recursive,
				supportedDesktopExts,
				remaining,
			)
			for _, file := range files {
				key := normalizeDesktopPath(file.FullPath)
				if _, exists := seenFiles[key]; exists {
					continue
				}
				targetGroup := group
				if classifiedGroup := groups[musicSourceIDForFile(file.Name)]; classifiedGroup != nil {
					targetGroup = classifiedGroup
				}
				if targetGroup == nil {
					continue
				}
				seenFiles[key] = struct{}{}
				targetGroup.Files = append(targetGroup.Files, SelectedFile{
					Path: file.FullPath,
					Name: file.Name,
					Size: file.Size,
				})
				remaining--
				if remaining <= 0 {
					result.Truncated = true
					break
				}
			}

			switch {
			case errors.Is(scanErr, service.ErrScanLimitReached):
				result.Truncated = true
			case scanErr != nil:
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s目录扫描失败：%s", source.Name, directory.Path))
			}
			if result.Truncated {
				break
			}
		}
		if result.Truncated {
			break
		}
	}

	for _, groupID := range groupOrder {
		group := groups[groupID]
		if group != nil && len(group.Files) > 0 {
			sort.Slice(group.Files, func(i, j int) bool {
				if strings.EqualFold(group.Files[i].Name, group.Files[j].Name) {
					return strings.ToLower(group.Files[i].Path) < strings.ToLower(group.Files[j].Path)
				}
				return strings.ToLower(group.Files[i].Name) < strings.ToLower(group.Files[j].Name)
			})
			result.Groups = append(result.Groups, *group)
		}
	}

	if result.Truncated {
		result.Warnings = append(result.Warnings, fmt.Sprintf("扫描结果已达到 %d 个文件上限或扫描超时，结果已截断。", maxFiles))
	} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.Truncated = true
		result.Warnings = append(result.Warnings, "扫描已超时，仅保留当前找到的文件。")
	} else if errors.Is(ctx.Err(), context.Canceled) {
		return FindLocalMusicResult{}, errors.New("本机音乐扫描已取消")
	}

	return result, nil
}

func musicSourceIDForFile(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".kgg", ".kgm", ".kgma", ".vpr":
		return "kugou"
	case ".ncm":
		return "netease"
	case ".kwm":
		return "kuwo"
	case ".mflac", ".mgg", ".qmc0", ".qmc2", ".qmc3", ".qmc4", ".qmc6", ".qmc8", ".qmcflac", ".qmcogg", ".tkm":
		return "qq"
	default:
		return ""
	}
}

func normalizeMusicSearchDirectories(directories []musicSearchDirectory) []musicSearchDirectory {
	result := make([]musicSearchDirectory, 0, len(directories))
	indexByPath := make(map[string]int)
	for _, directory := range directories {
		path := strings.TrimSpace(directory.Path)
		if path == "" || strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`) || !filepath.IsAbs(path) {
			continue
		}
		path = filepath.Clean(path)
		key := normalizeDesktopPath(path)
		if index, exists := indexByPath[key]; exists {
			result[index].Recursive = result[index].Recursive || directory.Recursive
			continue
		}
		indexByPath[key] = len(result)
		result = append(result, musicSearchDirectory{Path: path, Recursive: directory.Recursive})
	}
	return result
}
