package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"kugo-music-converter/internal/runtimebundle"
	"kugo-music-converter/internal/webview2bundle"
)

type releaseSelfTestReport struct {
	Success         bool   `json:"success"`
	Version         string `json:"version"`
	BuildDate       string `json:"buildDate"`
	CommitHash      string `json:"commitHash"`
	FFmpegReady     bool   `json:"ffmpegReady"`
	WebView2Bundled bool   `json:"webView2Bundled"`
	WebView2Ready   bool   `json:"webView2Ready"`
	DurationMillis  int64  `json:"durationMillis"`
	Error           string `json:"error,omitempty"`
}

func runReleaseSelfTest(arguments []string) (bool, int) {
	if len(arguments) == 0 || arguments[0] != "--release-self-test" {
		return false, 0
	}
	outputPath := ""
	for index := 1; index < len(arguments); index++ {
		if arguments[index] == "--output" && index+1 < len(arguments) {
			outputPath = strings.TrimSpace(arguments[index+1])
			index++
		}
	}

	startedAt := time.Now()
	report := releaseSelfTestReport{
		Version:    version,
		BuildDate:  buildDate,
		CommitHash: commitHash,
	}
	err := executeReleaseSelfTest(&report)
	if err != nil {
		report.Error = err.Error()
	}
	report.Success = err == nil
	report.DurationMillis = time.Since(startedAt).Milliseconds()
	if outputPath != "" {
		if writeErr := writeReleaseSelfTestReport(outputPath, report); writeErr != nil {
			return true, 2
		}
	}
	if err != nil {
		return true, 1
	}
	return true, 0
}

func executeReleaseSelfTest(report *releaseSelfTestReport) error {
	cacheRoot, err := os.MkdirTemp("", "kugo-release-self-test-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cacheRoot)

	ffmpegPayload, ffmpegHash := runtimebundle.FFmpegPayload()
	if len(ffmpegPayload) == 0 {
		return errors.New("正式构建未嵌入 FFmpeg")
	}
	ffmpegResult, err := runtimebundle.EnsureFFmpeg(cacheRoot, ffmpegPayload, ffmpegHash)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, ffmpegResult.Path, "-version")
	configureReleaseSelfTestCommand(command)
	output, err := command.CombinedOutput()
	if err != nil || ctx.Err() != nil || !strings.HasPrefix(strings.TrimSpace(string(output)), "ffmpeg version ") {
		return errors.New("内嵌 FFmpeg 无法正常执行")
	}
	report.FFmpegReady = true

	webView2Payload := webview2bundle.EmbeddedPayload()
	report.WebView2Bundled = len(webView2Payload.CAB) > 0
	if !report.WebView2Bundled {
		return nil
	}
	webView2Result, err := webview2bundle.EnsureRuntime(cacheRoot, webView2Payload)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(filepath.Join(webView2Result.BrowserPath, "msedgewebview2.exe")); statErr != nil || info.IsDir() {
		return errors.New("内嵌 WebView2 解压后缺少浏览器程序")
	}
	report.WebView2Ready = true
	return nil
}

func writeReleaseSelfTestReport(path string, report releaseSelfTestReport) error {
	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		return errors.New("自检报告路径必须是绝对路径")
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cleanPath, append(data, '\n'), 0o600)
}
