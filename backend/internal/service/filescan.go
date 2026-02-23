package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ScanFileInfo struct {
	Name     string `json:"name"`
	Ext      string `json:"ext"`
	Size     int64  `json:"size"`
	ModTime  string `json:"modTime"`
	FullPath string `json:"fullPath"`
}

type ScanFolderInfo struct {
	Path  string         `json:"path"`
	Files []ScanFileInfo `json:"files"`
}

type ScanResult struct {
	TotalFiles int              `json:"totalFiles"`
	TotalSize  int64            `json:"totalSize"`
	Folders    []ScanFolderInfo `json:"folders"`
}

func ParseExtFilter(raw string) map[string]struct{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	set := make(map[string]struct{})
	for _, part := range strings.Split(trimmed, ",") {
		ext := strings.ToLower(strings.TrimSpace(part))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		set[ext] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

// ErrScanLimitReached indicates that the file count limit was reached during scanning.
var ErrScanLimitReached = errors.New("scan: file count limit reached")

func ScanSingleFolder(path string, recursive bool, extFilter map[string]struct{}) ([]ScanFileInfo, int64, error) {
	return ScanSingleFolderCtx(context.Background(), path, recursive, extFilter, 0)
}

// ScanSingleFolderCtx scans a folder with context cancellation and an optional file count limit.
// If maxFiles <= 0, no limit is applied.
func ScanSingleFolderCtx(ctx context.Context, path string, recursive bool, extFilter map[string]struct{}, maxFiles int) ([]ScanFileInfo, int64, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, 0, err
	}

	entries := make([]ScanFileInfo, 0, 32)
	var totalSize int64

	walkFn := func(current string, d os.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if current != abs && !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if extFilter != nil {
			if _, ok := extFilter[ext]; !ok {
				return nil
			}
		}

		st, err := d.Info()
		if err != nil {
			return nil
		}

		entries = append(entries, ScanFileInfo{
			Name:     d.Name(),
			Ext:      ext,
			Size:     st.Size(),
			ModTime:  st.ModTime().Format(time.RFC3339),
			FullPath: current,
		})
		totalSize += st.Size()

		if maxFiles > 0 && len(entries) >= maxFiles {
			return filepath.SkipAll
		}
		return nil
	}

	if err := filepath.WalkDir(abs, walkFn); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return nil, 0, err
	}

	sort.Slice(entries, func(i, j int) bool {
		if strings.EqualFold(entries[i].Name, entries[j].Name) {
			return entries[i].FullPath < entries[j].FullPath
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries, totalSize, nil
}
