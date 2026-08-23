package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kugo-music-converter/internal/handler"
)

func TestSelectUpdateAssetsMatchesArchitectureAndVariant(t *testing.T) {
	assets := []handler.ReleaseAsset{
		{Name: "Kugo-Music-Converter-v0.6.0-windows-amd64-setup.exe", DownloadURL: officialAssetURL("Kugo-Music-Converter-v0.6.0-windows-amd64-setup.exe"), Size: 100},
		{Name: "Kugo-Music-Converter-v0.6.0-windows-amd64-setup.exe.sha256", DownloadURL: officialAssetURL("Kugo-Music-Converter-v0.6.0-windows-amd64-setup.exe.sha256"), Size: 100},
		{Name: "Kugo-Music-Converter-v0.6.0-windows-amd64-webview2-setup.exe", DownloadURL: officialAssetURL("Kugo-Music-Converter-v0.6.0-windows-amd64-webview2-setup.exe"), Size: 200},
		{Name: "Kugo-Music-Converter-v0.6.0-windows-amd64-webview2-setup.exe.sha256", DownloadURL: officialAssetURL("Kugo-Music-Converter-v0.6.0-windows-amd64-webview2-setup.exe.sha256"), Size: 100},
	}

	standard, _, err := selectUpdateAssets(assets, "amd64", false)
	if err != nil || standard.Name != assets[0].Name {
		t.Fatalf("standard selection = %+v, %v", standard, err)
	}
	bundled, _, err := selectUpdateAssets(assets, "amd64", true)
	if err != nil || bundled.Name != assets[2].Name {
		t.Fatalf("bundled selection = %+v, %v", bundled, err)
	}
}

func TestSelectUpdateAssetsRejectsMissingChecksumAndUntrustedURL(t *testing.T) {
	installerName := "Kugo-Music-Converter-v0.6.0-windows-amd64-setup.exe"
	if _, _, err := selectUpdateAssets([]handler.ReleaseAsset{{
		Name: installerName, DownloadURL: officialAssetURL(installerName), Size: 100,
	}}, "amd64", false); err == nil {
		t.Fatal("missing checksum was accepted")
	}
	if _, _, err := selectUpdateAssets([]handler.ReleaseAsset{
		{Name: installerName, DownloadURL: "https://example.com/update.exe", Size: 100},
		{Name: installerName + ".sha256", DownloadURL: officialAssetURL(installerName + ".sha256"), Size: 100},
	}, "amd64", false); err == nil {
		t.Fatal("untrusted installer URL was accepted")
	}
}

func TestParseAndMatchUpdateSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.exe")
	data := []byte("verified update")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	parsed, err := parseUpdateSHA256([]byte(hash+"  update.exe\n"), "update.exe")
	if err != nil || parsed != hash || !fileMatchesUpdateSHA256(path, parsed) {
		t.Fatalf("checksum parse/match = %q, %v", parsed, err)
	}
	if _, err := parseUpdateSHA256([]byte(hash+"  other.exe\n"), "update.exe"); err == nil {
		t.Fatal("checksum for a different filename was accepted")
	}
}

func TestDownloadOfficialReleaseAssetFallsBackAfterGitHubFailure(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	attemptedURLs := make([]string, 0, 2)
	http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attemptedURLs = append(attemptedURLs, request.URL.String())
		status := http.StatusServiceUnavailable
		body := "GitHub unavailable"
		if request.URL.Hostname() == "gh.h233.eu.org" {
			status = http.StatusOK
			body = "verified fallback asset"
		}
		return &http.Response{
			StatusCode:    status,
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
			Header:        make(http.Header),
			Request:       request,
		}, nil
	})

	assetName := "Kugo-Music-Converter-v0.6.0-windows-amd64-setup.exe"
	destination := filepath.Join(t.TempDir(), assetName)
	err := downloadOfficialReleaseAsset(t.Context(), handler.ReleaseAsset{
		Name:        assetName,
		DownloadURL: officialAssetURL(assetName),
		Size:        100,
	}, destination, 1024)
	if err != nil {
		t.Fatalf("downloadOfficialReleaseAsset() error = %v", err)
	}
	if len(attemptedURLs) != 2 || attemptedURLs[0] != officialAssetURL(assetName) || attemptedURLs[1] != updateDownloadProxyBase+officialAssetURL(assetName) {
		t.Fatalf("attempted URLs = %#v, want GitHub then fallback", attemptedURLs)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "verified fallback asset" {
		t.Fatalf("downloaded data = %q, %v", data, err)
	}
}

func TestDownloadOfficialReleaseAssetStopsAfterGitHubSuccess(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	attemptedHosts := make([]string, 0, 1)
	http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attemptedHosts = append(attemptedHosts, request.URL.Hostname())
		body := "official GitHub asset"
		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
			Header:        make(http.Header),
			Request:       request,
		}, nil
	})

	assetName := "Kugo-Music-Converter-v0.6.0-windows-amd64-setup.exe"
	destination := filepath.Join(t.TempDir(), assetName)
	err := downloadOfficialReleaseAsset(t.Context(), handler.ReleaseAsset{
		Name:        assetName,
		DownloadURL: officialAssetURL(assetName),
		Size:        100,
	}, destination, 1024)
	if err != nil {
		t.Fatalf("downloadOfficialReleaseAsset() error = %v", err)
	}
	if got := strings.Join(attemptedHosts, ","); got != "github.com" {
		t.Fatalf("attempted hosts = %q, want GitHub only", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func officialAssetURL(name string) string {
	return "https://github.com/skxxxkx666/Kugo-Music-Converter/releases/download/v0.6.0/" + name
}
