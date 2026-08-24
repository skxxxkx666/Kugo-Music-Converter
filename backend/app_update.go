package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kugo-music-converter/internal/handler"
	"kugo-music-converter/internal/webview2bundle"
)

const (
	maxUpdateInstallerBytes = int64(600 << 20)
	maxUpdateChecksumBytes  = int64(64 << 10)
	updateDownloadTimeout   = 30 * time.Minute
	updateDownloadProxyBase = "https://gh.h233.eu.org/"
)

var (
	updateTagPattern      = regexp.MustCompile(`^v\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)
	updateFileNamePattern = regexp.MustCompile(`^[0-9A-Za-z._-]+$`)
)

type UpdateInstallResult struct {
	Started       bool   `json:"started"`
	InstallerPath string `json:"installerPath,omitempty"`
	Message       string `json:"message"`
}

func (a *App) DownloadAndInstallUpdate(tagName string) (UpdateInstallResult, error) {
	if a.ctx == nil {
		return UpdateInstallResult{}, errors.New("桌面窗口尚未就绪")
	}
	a.mu.RLock()
	busy := a.activeTaskID != ""
	a.mu.RUnlock()
	if busy {
		return UpdateInstallResult{}, errors.New("请等待当前转换完成或取消任务后再安装更新")
	}

	tagName = strings.TrimSpace(tagName)
	if !updateTagPattern.MatchString(tagName) {
		return UpdateInstallResult{}, errors.New("更新版本号无效")
	}
	release, err := handler.CheckLatestRelease(a.ctx)
	if err != nil {
		return UpdateInstallResult{}, fmt.Errorf("重新确认最新版本失败: %w", err)
	}
	if release.Prerelease || release.TagName != tagName {
		return UpdateInstallResult{}, errors.New("更新信息已变化，请重新检查更新")
	}

	installer, checksum, err := selectUpdateAssets(release.Assets, runtime.GOARCH, len(webview2bundle.EmbeddedPayload().CAB) > 0)
	if err != nil {
		return UpdateInstallResult{}, err
	}
	choice, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:  wailsruntime.QuestionDialog,
		Title: "下载并安装更新",
		Message: fmt.Sprintf(
			"程序会先从项目官方 GitHub Release 获取 SHA-256 校验文件，再下载 %s（约 %s）。安装器直连失败时可使用 gh.h233.eu.org 转发同一官方地址；如果无法从 GitHub 获取校验文件，自动更新将停止。校验通过后才会启动安装器。\n\n安装器启动后应用将退出，请先保存其他工作。",
			installer.Name,
			formatCacheBytes(installer.Size),
		),
		Buttons:       []string{"下载并安装", "取消"},
		DefaultButton: "No",
		CancelButton:  "No",
	})
	if err != nil {
		return UpdateInstallResult{}, fmt.Errorf("打开更新确认窗口失败: %w", err)
	}
	if !messageDialogConfirmed(choice, "下载并安装") {
		return UpdateInstallResult{Message: "已取消更新"}, nil
	}

	cacheRoot, err := desktopCacheRoot()
	if err != nil {
		return UpdateInstallResult{}, err
	}
	updateDirectory := filepath.Join(cacheRoot, "updates", tagName)
	if err := os.MkdirAll(updateDirectory, 0o755); err != nil {
		return UpdateInstallResult{}, fmt.Errorf("创建更新缓存目录失败: %w", err)
	}
	installerPath := filepath.Join(updateDirectory, installer.Name)
	checksumPath := filepath.Join(updateDirectory, checksum.Name)

	ctx, cancel := context.WithTimeout(a.ctx, updateDownloadTimeout)
	defer cancel()
	if err := downloadOfficialReleaseAssetFromGitHub(ctx, checksum, checksumPath, maxUpdateChecksumBytes); err != nil {
		return UpdateInstallResult{}, fmt.Errorf("下载更新校验文件失败: %w", err)
	}
	checksumData, err := os.ReadFile(checksumPath)
	if err != nil {
		return UpdateInstallResult{}, fmt.Errorf("读取更新校验文件失败: %w", err)
	}
	expectedHash, err := parseUpdateSHA256(checksumData, installer.Name)
	if err != nil {
		return UpdateInstallResult{}, err
	}
	if !fileMatchesUpdateSHA256(installerPath, expectedHash) {
		if err := downloadOfficialReleaseAsset(ctx, installer, installerPath, maxUpdateInstallerBytes); err != nil {
			return UpdateInstallResult{}, fmt.Errorf("下载安装器失败: %w", err)
		}
	}
	if !fileMatchesUpdateSHA256(installerPath, expectedHash) {
		_ = os.Remove(installerPath)
		return UpdateInstallResult{}, errors.New("安装器 SHA-256 校验失败，已删除损坏文件")
	}

	command := exec.Command(installerPath)
	if err := command.Start(); err != nil {
		return UpdateInstallResult{}, fmt.Errorf("启动更新安装器失败: %w", err)
	}
	go func() { _ = command.Wait() }()
	time.AfterFunc(750*time.Millisecond, func() {
		wailsruntime.Quit(a.ctx)
	})
	return UpdateInstallResult{
		Started:       true,
		InstallerPath: installerPath,
		Message:       "安装器已启动，应用即将退出",
	}, nil
}

func selectUpdateAssets(assets []handler.ReleaseAsset, goarch string, bundledWebView2 bool) (handler.ReleaseAsset, handler.ReleaseAsset, error) {
	architecture := strings.ToLower(strings.TrimSpace(goarch))
	if architecture != "amd64" && architecture != "arm64" {
		return handler.ReleaseAsset{}, handler.ReleaseAsset{}, fmt.Errorf("当前处理器架构 %s 暂无自动更新安装器", goarch)
	}
	suffix := "-windows-" + architecture + "-setup.exe"
	if bundledWebView2 {
		suffix = "-windows-" + architecture + "-webview2-setup.exe"
	}

	var installer handler.ReleaseAsset
	for _, asset := range assets {
		if strings.HasSuffix(strings.ToLower(asset.Name), suffix) {
			installer = asset
			break
		}
	}
	if installer.Name == "" {
		variant := "标准版"
		if bundledWebView2 {
			variant = "内嵌 WebView2 版"
		}
		return handler.ReleaseAsset{}, handler.ReleaseAsset{}, fmt.Errorf("该版本未提供适用于 %s/%s 的安装器，请打开 Releases 手动更新", architecture, variant)
	}
	var checksum handler.ReleaseAsset
	for _, asset := range assets {
		if strings.EqualFold(asset.Name, installer.Name+".sha256") {
			checksum = asset
			break
		}
	}
	if checksum.Name == "" {
		return handler.ReleaseAsset{}, handler.ReleaseAsset{}, errors.New("该安装器缺少配套 SHA-256 校验文件，已拒绝自动更新")
	}
	for _, asset := range []handler.ReleaseAsset{installer, checksum} {
		if !updateFileNamePattern.MatchString(asset.Name) || filepath.Base(asset.Name) != asset.Name {
			return handler.ReleaseAsset{}, handler.ReleaseAsset{}, errors.New("更新资产文件名无效")
		}
		if err := validateOfficialReleaseAssetURL(asset.DownloadURL); err != nil {
			return handler.ReleaseAsset{}, handler.ReleaseAsset{}, err
		}
	}
	if installer.Size <= 0 || installer.Size > maxUpdateInstallerBytes {
		return handler.ReleaseAsset{}, handler.ReleaseAsset{}, errors.New("更新安装器大小异常")
	}
	return installer, checksum, nil
}

func validateOfficialReleaseAssetURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return errors.New("更新资产不是项目官方 GitHub Release 地址")
	}
	const releasePathPrefix = "/skxxxkx666/Kugo-Music-Converter/releases/download/"
	if !strings.HasPrefix(parsed.EscapedPath(), releasePathPrefix) {
		return errors.New("更新资产不是项目官方 GitHub Release 地址")
	}
	return nil
}

func downloadOfficialReleaseAsset(ctx context.Context, asset handler.ReleaseAsset, destination string, maximumBytes int64) error {
	if err := validateOfficialReleaseAssetURL(asset.DownloadURL); err != nil {
		return err
	}
	downloadURLs := []string{
		asset.DownloadURL,
		updateDownloadProxyBase + asset.DownloadURL,
	}
	failures := make([]error, 0, len(downloadURLs))
	for index, downloadURL := range downloadURLs {
		if err := downloadReleaseAssetFromURL(ctx, downloadURL, destination, maximumBytes); err == nil {
			return nil
		} else if index == 0 {
			failures = append(failures, fmt.Errorf("GitHub: %w", err))
		} else {
			failures = append(failures, fmt.Errorf("备用下载地址: %w", err))
		}
	}
	return fmt.Errorf("GitHub 与备用下载地址均失败: %w", errors.Join(failures...))
}

func downloadOfficialReleaseAssetFromGitHub(ctx context.Context, asset handler.ReleaseAsset, destination string, maximumBytes int64) error {
	// The checksum is the trust anchor for a proxied installer and must never use
	// the same fallback proxy or follow a redirect to it.
	if err := validateOfficialReleaseAssetURL(asset.DownloadURL); err != nil {
		return err
	}
	if err := downloadReleaseAssetFromURL(ctx, asset.DownloadURL, destination, maximumBytes); err != nil {
		return fmt.Errorf("GitHub: %w", err)
	}
	return nil
}

func downloadReleaseAssetFromURL(ctx context.Context, downloadURL string, destination string, maximumBytes int64) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "Kugo-Music-Converter-Updater")
	allowProxyRedirect := strings.EqualFold(request.URL.Hostname(), "gh.h233.eu.org")
	client := &http.Client{
		CheckRedirect: func(redirectRequest *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("更新下载重定向次数过多")
			}
			host := strings.ToLower(redirectRequest.URL.Hostname())
			if host == "gh.h233.eu.org" && !allowProxyRedirect {
				return errors.New("官方更新下载不允许重定向到备用地址")
			}
			if host != "github.com" && host != "gh.h233.eu.org" && !strings.HasSuffix(host, ".githubusercontent.com") {
				return errors.New("更新下载被重定向到未允许的地址")
			}
			return nil
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub 返回 HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maximumBytes {
		return errors.New("更新下载内容超过大小限制")
	}

	tempFile, err := os.CreateTemp(filepath.Dir(destination), filepath.Base(destination)+"-*.part")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	keep := false
	defer func() {
		_ = tempFile.Close()
		if !keep {
			_ = os.Remove(tempPath)
		}
	}()
	written, copyErr := io.Copy(tempFile, io.LimitReader(response.Body, maximumBytes+1))
	if copyErr != nil {
		return copyErr
	}
	if written > maximumBytes {
		return errors.New("更新下载内容超过大小限制")
	}
	if err := tempFile.Sync(); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	_ = os.Remove(destination)
	if err := os.Rename(tempPath, destination); err != nil {
		return err
	}
	keep = true
	return nil
}

func parseUpdateSHA256(data []byte, expectedFileName string) (string, error) {
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "", errors.New("更新 SHA-256 校验文件为空")
	}
	hash := strings.ToLower(strings.TrimSpace(fields[0]))
	if len(hash) != sha256.Size*2 {
		return "", errors.New("更新 SHA-256 格式无效")
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return "", errors.New("更新 SHA-256 格式无效")
	}
	if len(fields) > 1 {
		fileName := strings.TrimPrefix(strings.TrimSpace(fields[1]), "*")
		if fileName != "" && !strings.EqualFold(fileName, expectedFileName) {
			return "", errors.New("更新 SHA-256 对应的文件名不匹配")
		}
	}
	return hash, nil
}

func fileMatchesUpdateSHA256(path string, expectedHash string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}
	return hex.EncodeToString(hasher.Sum(nil)) == expectedHash
}
