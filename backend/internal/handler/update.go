package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"kugo-music-converter/internal/logger"
)

const (
	githubLatestReleaseURL = "https://api.github.com/repos/skxxxkx666/Kugo-Music-Converter/releases/latest"
	updateCacheTTL         = 30 * time.Minute
	githubRequestTimeout   = 15 * time.Second
)

// githubMirrors are fallback URLs when api.github.com is unreachable.
var githubMirrors = []string{
	githubLatestReleaseURL,
	"https://ghfast.top/https://api.github.com/repos/skxxxkx666/Kugo-Music-Converter/releases/latest",
	"https://gh-proxy.com/https://api.github.com/repos/skxxxkx666/Kugo-Music-Converter/releases/latest",
}

type updateCacheEntry struct {
	data      *releaseInfo
	fetchedAt time.Time
}

type releaseInfo struct {
	TagName     string `json:"tagName"`
	HtmlURL     string `json:"htmlUrl"`
	Body        string `json:"body"`
	PublishedAt string `json:"publishedAt"`
	Prerelease  bool   `json:"prerelease"`
}

type ghReleaseResponse struct {
	TagName     string `json:"tag_name"`
	HtmlURL     string `json:"html_url"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	Prerelease  bool   `json:"prerelease"`
}

var (
	updateCache   *updateCacheEntry
	updateCacheMu sync.RWMutex
)

// HandleCheckUpdate proxies the GitHub release API through the backend,
// which works around browser-level network restrictions to api.github.com.
// The backend respects HTTP_PROXY / HTTPS_PROXY environment variables and
// caches results for 30 minutes to avoid rate limiting.
func (h *ConvertHandler) HandleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	// Check cache first
	updateCacheMu.RLock()
	cached := updateCache
	updateCacheMu.RUnlock()

	if cached != nil && cached.data != nil && time.Since(cached.fetchedAt) < updateCacheTTL {
		writeJSON(w, http.StatusOK, cached.data)
		return
	}

	// Fetch from GitHub (try mirrors in order)
	release, err := fetchLatestRelease(r.Context())
	if err != nil {
		logger.Warnf("更新检测失败: %v", err)
		// If we have stale cache, return it rather than an error
		if cached != nil && cached.data != nil {
			writeJSON(w, http.StatusOK, cached.data)
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "无法连接 GitHub，请检查网络连接或代理设置",
		})
		return
	}

	// Update cache
	updateCacheMu.Lock()
	updateCache = &updateCacheEntry{data: release, fetchedAt: time.Now()}
	updateCacheMu.Unlock()

	writeJSON(w, http.StatusOK, release)
}

func fetchLatestRelease(parentCtx context.Context) (*releaseInfo, error) {
	var lastErr error

	for _, mirror := range githubMirrors {
		ctx, cancel := context.WithTimeout(parentCtx, githubRequestTimeout)
		result, err := fetchFromURL(ctx, mirror)
		cancel()
		if err != nil {
			lastErr = err
			logger.Debugf("更新检测 mirror 请求失败 (%s): %v", mirror, err)
			continue
		}
		return result, nil
	}

	return nil, lastErr
}

func fetchFromURL(ctx context.Context, url string) (*releaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Kugo-Music-Converter")

	// http.DefaultClient respects HTTP_PROXY / HTTPS_PROXY env vars
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, &httpError{statusCode: resp.StatusCode, body: string(body)}
	}

	var ghRelease ghReleaseResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&ghRelease); err != nil {
		return nil, err
	}

	return &releaseInfo{
		TagName:     ghRelease.TagName,
		HtmlURL:     ghRelease.HtmlURL,
		Body:        ghRelease.Body,
		PublishedAt: ghRelease.PublishedAt,
		Prerelease:  ghRelease.Prerelease,
	}, nil
}

type httpError struct {
	statusCode int
	body       string
}

func (e *httpError) Error() string {
	return "GitHub API returned HTTP " + http.StatusText(e.statusCode)
}
