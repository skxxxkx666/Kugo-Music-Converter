package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"kugo-music-converter/internal/config"
	"kugo-music-converter/internal/handler"
	"kugo-music-converter/internal/logger"
)

var (
	version    = "v0.3.9"
	buildDate  = "unknown"
	commitHash = "unknown"
	appEnv     = "unknown"
)

func main() {
	configPath := flag.String("config", "", "配置文件路径")
	showHelp := flag.Bool("help", false, "显示帮助")
	showVersion := flag.Bool("version", false, "显示版本信息")
	showEnv := flag.Bool("env", false, "显示运行环境")
	addr := flag.String("addr", ":8080", "服务监听地址")
	ffmpegBin := flag.String("ffmpeg", "ffmpeg", "ffmpeg 可执行文件路径")

	flag.Parse()

	addrSet := false
	ffmpegSet := false
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr":
			addrSet = true
		case "ffmpeg":
			ffmpegSet = true
		}
	})

	if *showHelp {
		printHelp()
		return
	}
	if *showVersion {
		printVersion()
		return
	}
	if *showEnv {
		printEnv()
		return
	}

	cfg, err := config.LoadConfig(*configPath, *addr, *ffmpegBin, addrSet, ffmpegSet)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	cleanupStaleTempFiles(1 * time.Hour)

	logger.Infof("启动服务，监听地址: %s", cfg.Addr)
	logger.Infof("FFmpeg 路径: %s", cfg.FFmpegBin)
	logger.Infof("单文件最大大小: %d bytes", cfg.MaxFileSize)
	logger.Infof("最大文件数: %d", cfg.MaxFiles)

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-shutdownCtx.Done()
		logger.Warnf("收到退出信号，开始优雅关闭服务...")
	}()

	if err := handler.StartServer(shutdownCtx, cfg, version); err != nil {
		logger.Errorf("服务启动失败: %v", err)
		os.Exit(1)
	}

	logger.Infof("服务已关闭")
}

func cleanupStaleTempFiles(maxAge time.Duration) {
	if maxAge <= 0 {
		maxAge = time.Hour
	}
	tmpDir := os.TempDir()
	if strings.TrimSpace(tmpDir) == "" {
		return
	}

	patterns := []string{
		"kgm_dec_*",
		"kgg_dec_*",
		"ncm_dec_*",
		"kgdb_dec_*",
		"kgg-upload-*",
		"kgg-raw-audio-*",
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(tmpDir, pattern))
		if err != nil {
			logger.Warnf("临时文件模式匹配失败: %s (%v)", pattern, err)
			continue
		}
		for _, candidate := range matches {
			st, err := os.Stat(candidate)
			if err != nil || st.IsDir() {
				continue
			}
			if st.ModTime().After(cutoff) {
				continue
			}
			if err := os.Remove(candidate); err != nil {
				logger.Warnf("删除残留临时文件失败: %s (%v)", candidate, err)
				continue
			}
			removed++
		}
	}

	if removed > 0 {
		logger.Infof("已清理 %d 个残留临时文件（阈值: %s）", removed, maxAge)
	} else {
		logger.Debugf("未发现需要清理的残留临时文件")
	}
}

func printHelp() {
	fmt.Println("Kugo 音频解密转换服务")
	fmt.Println("用法: server [选项]")
	fmt.Println()
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  server --addr :8080 --ffmpeg tools/ffmpeg.exe")
}

func printVersion() {
	fmt.Printf("Kugo 音频解密转换服务\n")
	fmt.Printf("版本: %s\n", version)
	fmt.Printf("构建日期: %s\n", buildDate)
	fmt.Printf("Git 提交: %s\n", commitHash)
	fmt.Printf("Go 版本: %s\n", runtime.Version())
	fmt.Printf("系统架构: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func printEnv() {
	fmt.Printf("当前运行环境: %s\n", appEnv)
}
