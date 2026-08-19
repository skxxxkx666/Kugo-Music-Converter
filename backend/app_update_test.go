package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
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

func officialAssetURL(name string) string {
	return "https://github.com/skxxxkx666/Kugo-Music-Converter/releases/download/v0.6.0/" + name
}
