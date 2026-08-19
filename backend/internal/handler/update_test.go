package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchFromURLParsesRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept header = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name":"v0.6.0",
			"html_url":"https://github.com/skxxxkx666/Kugo-Music-Converter/releases/tag/v0.6.0",
			"body":"release notes",
			"published_at":"2026-08-17T00:00:00Z",
			"prerelease":false,
			"assets":[{
				"name":"Kugo-Music-Converter-v0.6.0-windows-amd64-setup.exe",
				"browser_download_url":"https://github.com/skxxxkx666/Kugo-Music-Converter/releases/download/v0.6.0/Kugo-Music-Converter-v0.6.0-windows-amd64-setup.exe",
				"size":12345
			}]
		}`))
	}))
	defer server.Close()

	release, err := fetchFromURL(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("fetchFromURL() error = %v", err)
	}
	if release.TagName != "v0.6.0" || release.Prerelease {
		t.Fatalf("release = %+v", release)
	}
	if release.HtmlURL == "" || release.PublishedAt == "" {
		t.Fatalf("release is missing link or publish date: %+v", release)
	}
	if len(release.Assets) != 1 || release.Assets[0].Size != 12345 || release.Assets[0].DownloadURL == "" {
		t.Fatalf("release assets = %+v", release.Assets)
	}
}

func TestFetchFromURLRejectsNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	if _, err := fetchFromURL(context.Background(), server.URL); err == nil {
		t.Fatal("fetchFromURL() error = nil, want HTTP status error")
	}
}
