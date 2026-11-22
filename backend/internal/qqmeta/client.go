// Package qqmeta resolves QQ Music track metadata by songmid and writes the
// tags (plus cover art) into already-converted audio files.
//
// The metadata itself is not stored inside QQ Music encrypted files. The
// musicex container only carries a songmid in its footer, so the tag values are
// fetched from QQ Music's public web endpoints and the cover from its CDN.
package qqmeta

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDetailURL = "https://u.y.qq.com/cgi-bin/musicu.fcg"
	defaultCoverURL  = "https://y.gtimg.cn/music/photo_new/T002R%dx%dM000%s.jpg"
	defaultSingerURL = "https://y.gtimg.cn/music/photo_new/T001R%dx%dM000%s.jpg"

	maxResponseBytes = 1 << 20
	maxCoverBytes    = 8 << 20
	minCoverBytes    = 1024
	requestTimeout   = 20 * time.Second
)

// Track is the metadata written into converted files.
type Track struct {
	SongMID   string
	Title     string
	Artists   []string
	Album     string
	AlbumMID  string
	SingerMID string
	Track     int
	Disc      int
	Year      string
	Genre     string
}

// Client queries QQ Music's public metadata endpoints.
type Client struct {
	httpClient *http.Client
	detailURL  string
	coverURL   string
	singerURL  string
}

// NewClient returns a client with bounded timeouts and a size-limited reader.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: requestTimeout},
		detailURL:  defaultDetailURL,
		coverURL:   defaultCoverURL,
		singerURL:  defaultSingerURL,
	}
}

type detailResponse struct {
	Req struct {
		Code int `json:"code"`
		Data struct {
			Track struct {
				Name       string   `json:"name"`
				Mid        string   `json:"mid"`
				Singer     []singer `json:"singer"`
				Album      album    `json:"album"`
				IndexAlbum int      `json:"index_album"`
				IndexCD    int      `json:"index_cd"`
			} `json:"track_info"`
			Info struct {
				Genre   contentNode `json:"genre"`
				PubTime contentNode `json:"pub_time"`
			} `json:"info"`
		} `json:"data"`
	} `json:"req"`
}

type singer struct {
	Name string `json:"name"`
	Mid  string `json:"mid"`
}

type album struct {
	Name       string `json:"name"`
	Mid        string `json:"mid"`
	TimePublic string `json:"time_public"`
}

type contentNode struct {
	Content []struct {
		Value string `json:"value"`
	} `json:"content"`
}

func (n contentNode) first() string {
	if len(n.Content) == 0 {
		return ""
	}
	return strings.TrimSpace(n.Content[0].Value)
}

// Lookup resolves a songmid to track metadata.
func (c *Client) Lookup(ctx context.Context, songMID string) (*Track, error) {
	songMID = strings.TrimSpace(songMID)
	if songMID == "" {
		return nil, errors.New("qqmeta: empty songmid")
	}

	payload := map[string]any{
		"comm": map[string]string{"ct": "24", "cv": "0"},
		"req": map[string]any{
			"module": "music.pf_song_detail_svr",
			"method": "get_song_detail_yqq",
			"param":  map[string]string{"song_mid": songMID},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.detailURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	setCommonHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qqmeta: detail request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qqmeta: detail status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, err
	}
	var parsed detailResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("qqmeta: detail response invalid: %w", err)
	}
	if parsed.Req.Code != 0 {
		return nil, fmt.Errorf("qqmeta: detail code %d", parsed.Req.Code)
	}

	info := parsed.Req.Data.Track
	if strings.TrimSpace(info.Name) == "" || strings.TrimSpace(info.Mid) == "" {
		return nil, errors.New("qqmeta: track not found")
	}

	track := &Track{
		SongMID:  info.Mid,
		Title:    strings.TrimSpace(info.Name),
		Album:    strings.TrimSpace(info.Album.Name),
		AlbumMID: strings.TrimSpace(info.Album.Mid),
		Track:    info.IndexAlbum,
		Disc:     info.IndexCD,
		Genre:    parsed.Req.Data.Info.Genre.first(),
	}
	for _, s := range info.Singer {
		if name := strings.TrimSpace(s.Name); name != "" {
			track.Artists = append(track.Artists, name)
		}
		if track.SingerMID == "" {
			track.SingerMID = strings.TrimSpace(s.Mid)
		}
	}
	public := strings.TrimSpace(info.Album.TimePublic)
	if public == "" {
		public = parsed.Req.Data.Info.PubTime.first()
	}
	if len(public) >= 4 {
		if _, err := strconv.Atoi(public[:4]); err == nil {
			track.Year = public[:4]
		}
	}
	return track, nil
}

// Cover downloads the album cover (falling back to the artist picture).
func (c *Client) Cover(ctx context.Context, track *Track, size int) ([]byte, error) {
	if track == nil {
		return nil, errors.New("qqmeta: nil track")
	}
	if size <= 0 {
		size = 500
	}

	var candidates []string
	if track.AlbumMID != "" {
		candidates = append(candidates, fmt.Sprintf(c.coverURL, size, size, track.AlbumMID))
	}
	if track.SingerMID != "" {
		candidates = append(candidates, fmt.Sprintf(c.singerURL, size, size, track.SingerMID))
	}
	if len(candidates) == 0 {
		return nil, errors.New("qqmeta: no cover mid")
	}

	var lastErr error
	for _, rawURL := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			lastErr = err
			continue
		}
		setCommonHeaders(req)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxCoverBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK || len(data) < minCoverBytes {
			lastErr = fmt.Errorf("qqmeta: cover status %d", resp.StatusCode)
			continue
		}
		if !isJPEG(data) && !isPNG(data) {
			lastErr = errors.New("qqmeta: cover is not an image")
			continue
		}
		return data, nil
	}
	return nil, lastErr
}

func setCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	req.Header.Set("Referer", "https://y.qq.com/")
}

func isJPEG(data []byte) bool {
	return len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF
}

func isPNG(data []byte) bool {
	return len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A})
}

// SupportsEmbeddedCover reports whether ffmpeg can write a cover stream into
// the given output extension.
func SupportsEmbeddedCover(ext string) bool {
	switch strings.ToLower(ext) {
	case ".mp3", ".flac", ".m4a":
		return true
	}
	return false
}

// Apply rewrites audioPath in place with the given tags and, when the container
// supports it, the cover image. The audio stream is copied, never re-encoded.
func (c *Client) Apply(ctx context.Context, ffmpegPath, audioPath string, track *Track, cover []byte) error {
	if track == nil || strings.TrimSpace(track.Title) == "" {
		return errors.New("qqmeta: no metadata to write")
	}
	ext := strings.ToLower(filepath.Ext(audioPath))
	withCover := len(cover) > 0 && SupportsEmbeddedCover(ext)

	tmpDir := filepath.Dir(audioPath)
	tmpOut, err := os.CreateTemp(tmpDir, ".qqmeta-*"+ext)
	if err != nil {
		return fmt.Errorf("qqmeta: create temp output: %w", err)
	}
	tmpPath := tmpOut.Name()
	_ = tmpOut.Close()
	_ = os.Remove(tmpPath)
	defer os.Remove(tmpPath)

	var coverPath string
	if withCover {
		coverFile, err := os.CreateTemp("", "qqmeta-cover-*.jpg")
		if err != nil {
			return fmt.Errorf("qqmeta: create temp cover: %w", err)
		}
		coverPath = coverFile.Name()
		if _, err := coverFile.Write(cover); err != nil {
			_ = coverFile.Close()
			return fmt.Errorf("qqmeta: write temp cover: %w", err)
		}
		_ = coverFile.Close()
		defer os.Remove(coverPath)
	}

	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-nostdin", "-i", audioPath}
	if withCover {
		args = append(args, "-i", coverPath)
	}
	args = append(args, "-map", "0:a:0")
	if withCover {
		args = append(args, "-map", "1:v:0", "-disposition:v:0", "attached_pic")
	}
	args = append(args, "-c", "copy", "-map_metadata", "0")
	if ext == ".mp3" {
		args = append(args, "-id3v2_version", "3")
	}
	for _, tag := range buildTags(track) {
		if tag[1] != "" {
			args = append(args, "-metadata", tag[0]+"="+tag[1])
		}
	}
	if withCover {
		args = append(args,
			"-metadata:s:v:0", "title=Album cover",
			"-metadata:s:v:0", "comment=Cover (front)",
		)
	}
	args = append(args, tmpPath)

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("qqmeta: ffmpeg tag write failed: %s", strings.TrimSpace(stderr.String()))
	}
	stat, err := os.Stat(tmpPath)
	if err != nil || stat.Size() == 0 {
		return errors.New("qqmeta: ffmpeg produced no output")
	}
	if err := os.Rename(tmpPath, audioPath); err != nil {
		return fmt.Errorf("qqmeta: replace output: %w", err)
	}
	return nil
}

func buildTags(track *Track) [][2]string {
	artists := strings.Join(track.Artists, "/")
	tags := [][2]string{
		{"title", track.Title},
		{"artist", artists},
		{"album", track.Album},
		{"album_artist", artists},
		{"date", track.Year},
		{"genre", track.Genre},
	}
	if track.Track > 0 {
		tags = append(tags, [2]string{"track", strconv.Itoa(track.Track)})
	}
	if track.Disc > 0 {
		tags = append(tags, [2]string{"disc", strconv.Itoa(track.Disc)})
	}
	return tags
}
