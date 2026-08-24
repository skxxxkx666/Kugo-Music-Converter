package handler

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"kugo-music-converter/internal/algo/qmcfile"
	"kugo-music-converter/internal/qmckey"
	"kugo-music-converter/internal/service"
)

type qmcBatchKey struct {
	ekey string
	err  error
}

func isModernQMCExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".mflac" || ext == ".mgg"
}

func (h *ConvertHandler) resolveQMCBatchKeys(ctx context.Context, items []service.BatchItem) map[string]qmcBatchKey {
	resolved := make(map[string]qmcBatchKey)
	resources := make([]qmckey.Resource, 0)
	paths := make([]string, 0)
	resourceIndex := make(map[string]int)

	for _, item := range items {
		if !isModernQMCExt(item.Name) {
			continue
		}
		info, err := qmcfile.Inspect(item.Path)
		if err != nil {
			resolved[item.Path] = qmcBatchKey{err: NewAppError(ErrQMCContainerInvalid, "QQ 音乐文件尾部无效", nil)}
			continue
		}
		switch info.Kind {
		case qmcfile.KindLegacy, qmcfile.KindQTag:
			continue
		case qmcfile.KindSTag:
			resolved[item.Path] = qmcBatchKey{err: NewAppError(ErrQMCSTagUnsupported, "STag 文件缺少可用于 GetEVkey 的资源信息", nil)}
		case qmcfile.KindMusicEx:
			if info.Metadata == nil {
				resolved[item.Path] = qmcBatchKey{err: NewAppError(ErrQMCContainerInvalid, "musicex 元数据缺失", nil)}
				continue
			}
			resource := qmckey.Resource{
				SongID:   info.Metadata.SongID,
				MediaMid: info.Metadata.MediaMID,
				Filename: info.Metadata.ResourceFilename,
			}
			key := resource.MediaMid + "\x00" + resource.Filename
			if index, ok := resourceIndex[key]; ok {
				paths[index] += "\x00" + item.Path
				continue
			}
			resourceIndex[key] = len(resources)
			resources = append(resources, resource)
			paths = append(paths, item.Path)
		default:
			resolved[item.Path] = qmcBatchKey{err: NewAppError(ErrQMCContainerInvalid, "未知 QQ 音乐文件容器", nil)}
		}
	}

	if len(resources) == 0 {
		return resolved
	}
	if h.qmcKeyResolver == nil {
		for _, groupedPaths := range paths {
			for _, path := range strings.Split(groupedPaths, "\x00") {
				resolved[path] = qmcBatchKey{err: NewAppError(ErrQMCKeyUnavailable, "QQ 音乐取钥能力不可用", nil)}
			}
		}
		return resolved
	}

	results := h.qmcKeyResolver.ResolveBatch(ctx, resources)
	for i, groupedPaths := range paths {
		entry := qmcBatchKey{err: NewAppError(ErrQMCKeyUnavailable, "QQ 音乐取钥未返回结果", nil)}
		if i < len(results) {
			if results[i].Err != nil {
				entry.err = mapQMCKeyError(results[i].Err)
			} else if strings.TrimSpace(results[i].EKey) == "" {
				entry.err = NewAppError(ErrQMCKeyUnavailable, "QQ 音乐返回空密钥", nil)
			} else {
				entry = qmcBatchKey{ekey: results[i].EKey}
			}
		}
		for _, path := range strings.Split(groupedPaths, "\x00") {
			resolved[path] = entry
		}
	}
	return resolved
}

func mapQMCKeyError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return NewAppError(ErrCancelled, "QQ 音乐取钥已取消", nil)
	case errors.Is(err, context.DeadlineExceeded):
		return NewAppError(ErrQMCNetwork, "QQ 音乐取钥超时", nil)
	case errors.Is(err, qmckey.ErrNotLoggedIn), errors.Is(err, qmckey.ErrUnavailable):
		return NewAppError(ErrQMCLoginRequired, "未找到可用的 QQ 音乐登录会话", nil)
	case errors.Is(err, qmckey.ErrSessionExpired):
		return NewAppError(ErrQMCSessionExpired, "QQ 音乐登录会话已过期", nil)
	case errors.Is(err, qmckey.ErrEntitlement):
		return NewAppError(ErrQMCEntitlement, "当前 QQ 音乐账号无权获取此资源密钥", nil)
	case errors.Is(err, qmckey.ErrNetwork):
		return NewAppError(ErrQMCNetwork, "无法连接 QQ 音乐官方取钥接口", nil)
	case errors.Is(err, qmckey.ErrProtocol):
		return NewAppError(ErrQMCKeyUnavailable, "QQ 音乐取钥响应无效", nil)
	default:
		return NewAppError(ErrQMCKeyUnavailable, fmt.Sprintf("QQ 音乐取钥失败 (%T)", err), nil)
	}
}
