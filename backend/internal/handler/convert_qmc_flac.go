package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"kugo-music-converter/internal/logger"
	"kugo-music-converter/internal/service"
)

func (h *ConvertHandler) ensureValidModernFLAC(ctx context.Context, outputPath string) error {
	if err := service.ValidateAudioFileStrict(ctx, h.ffmpegPath, outputPath); err == nil {
		return nil
	} else {
		logger.Warnf("MFLAC 解密流包含无效尾包，开始重建 FLAC: file=%s", filepath.Base(outputPath))
	}

	repairedFile, err := os.CreateTemp(filepath.Dir(outputPath), ".qmc-repaired-*.flac")
	if err != nil {
		return NewAppError(ErrTranscodeFailed, "创建 FLAC 修复文件失败", err)
	}
	repaired := repairedFile.Name()
	if err := repairedFile.Close(); err != nil {
		removeQuiet(repaired)
		return NewAppError(ErrTranscodeFailed, "关闭 FLAC 修复文件失败", err)
	}
	defer removeQuiet(repaired)

	if err := service.RepairFLACFile(ctx, h.ffmpegPath, outputPath, repaired); err != nil {
		return NewAppError(ErrTranscodeFailed, "修复 MFLAC 解密后的尾部坏包失败", err)
	}
	if err := service.ValidateAudioFileStrict(ctx, h.ffmpegPath, repaired); err != nil {
		return NewAppError(ErrTranscodeFailed, "修复后的 FLAC 严格校验失败", err)
	}

	backup := outputPath + ".qmc-unrepaired"
	removeQuiet(backup)
	if err := os.Rename(outputPath, backup); err != nil {
		return NewAppError(ErrTranscodeFailed, "备份待修复 FLAC 失败", err)
	}
	if err := os.Rename(repaired, outputPath); err != nil {
		_ = os.Rename(backup, outputPath)
		return NewAppError(ErrTranscodeFailed, fmt.Sprintf("启用修复后的 FLAC 失败: %s", filepath.Base(outputPath)), err)
	}
	removeQuiet(backup)
	logger.Infof("MFLAC 尾部坏包已重建为严格可解码 FLAC: file=%s", filepath.Base(outputPath))
	return nil
}
