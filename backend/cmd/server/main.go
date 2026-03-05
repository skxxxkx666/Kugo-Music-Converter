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
	version    = "v0.3.8"
	buildDate  = "unknown"
	commitHash = "unknown"
	appEnv     = "unknown"
)

func main() {
	configPath := flag.String("config", "", "閰嶇疆鏂囦欢璺緞")
	showHelp := flag.Bool("help", false, "鏄剧ず甯姪")
	showVersion := flag.Bool("version", false, "鏄剧ず鐗堟湰淇℃伅")
	showEnv := flag.Bool("env", false, "鏄剧ず杩愯鐜")
	addr := flag.String("addr", ":8080", "鏈嶅姟鐩戝惉鍦板潃")
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
		log.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	cleanupStaleTempFiles(1 * time.Hour)

	logger.Infof("鍚姩鏈嶅姟锛岀洃鍚湴鍧€: %s", cfg.Addr)
	logger.Infof("FFmpeg 璺緞: %s", cfg.FFmpegBin)
	logger.Infof("鍗曟枃浠舵渶澶уぇ灏? %d bytes", cfg.MaxFileSize)
	logger.Infof("鏈€澶ф枃浠舵暟: %d", cfg.MaxFiles)

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-shutdownCtx.Done()
		logger.Warnf("鏀跺埌閫€鍑轰俊鍙凤紝寮€濮嬩紭闆呭叧闂湇鍔?..")
	}()

	if err := handler.StartServer(shutdownCtx, cfg, version); err != nil {
		logger.Errorf("鏈嶅姟鍚姩澶辫触: %v", err)
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
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(tmpDir, pattern))
		if err != nil {
			logger.Warnf("涓存椂鏂囦欢妯″紡鍖归厤澶辫触: %s (%v)", pattern, err)
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
				logger.Warnf("鍒犻櫎娈嬬暀涓存椂鏂囦欢澶辫触: %s (%v)", candidate, err)
				continue
			}
			removed++
		}
	}

	if removed > 0 {
		logger.Infof("已清理 %d 个残留临时文件（阈值: %s）", removed, maxAge)
	} else {
		logger.Debugf("鏈彂鐜伴渶瑕佹竻鐞嗙殑娈嬬暀涓存椂鏂囦欢")
	}
}

func printHelp() {
	fmt.Println("Kugo 闊抽瑙ｅ瘑杞崲鏈嶅姟")
	fmt.Println("鐢ㄦ硶: server [閫夐」]")
	fmt.Println()
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("绀轰緥:")
	fmt.Println("  server --addr :8080 --ffmpeg tools/ffmpeg.exe")
}

func printVersion() {
	fmt.Printf("Kugo 闊抽瑙ｅ瘑杞崲鏈嶅姟\n")
	fmt.Printf("鐗堟湰: %s\n", version)
	fmt.Printf("鏋勫缓鏃ユ湡: %s\n", buildDate)
	fmt.Printf("Git 鎻愪氦: %s\n", commitHash)
	fmt.Printf("Go 鐗堟湰: %s\n", runtime.Version())
	fmt.Printf("绯荤粺鏋舵瀯: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func printEnv() {
	fmt.Printf("褰撳墠杩愯鐜: %s\n", appEnv)
}

