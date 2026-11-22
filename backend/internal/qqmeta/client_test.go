package qqmeta

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLookupParsesTrack(t *testing.T) {
	response := map[string]any{
		"req": map[string]any{
			"code": 0,
			"data": map[string]any{
				"track_info": map[string]any{
					"name":        "Test Song",
					"mid":         "000testsongmid",
					"index_album": 4,
					"index_cd":    0,
					"singer": []map[string]any{
						{"name": "Test Artist", "mid": "000testsinger0"},
					},
					"album": map[string]any{
						"name":        "Test Album",
						"mid":         "000testalbum00",
						"time_public": "2024-01-02",
					},
				},
				"info": map[string]any{
					"genre": map[string]any{
						"content": []map[string]any{{"value": "Pop"}},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient()
	client.httpClient = server.Client()
	client.detailURL = server.URL
	track, err := client.Lookup(context.Background(), "000testsongmid")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if track.Title != "Test Song" || track.Album != "Test Album" || track.AlbumMID != "000testalbum00" {
		t.Fatalf("unexpected track: %+v", track)
	}
	if len(track.Artists) != 1 || track.Artists[0] != "Test Artist" {
		t.Fatalf("unexpected artists: %v", track.Artists)
	}
	if track.Track != 4 || track.Year != "2024" || track.Genre != "Pop" {
		t.Fatalf("unexpected numbers: %+v", track)
	}
	if track.SingerMID != "000testsinger0" {
		t.Fatalf("unexpected singer mid: %q", track.SingerMID)
	}
}

func TestLookupMissingTrack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"req":{"code":0,"data":{}}}`))
	}))
	defer server.Close()

	client := NewClient()
	client.httpClient = server.Client()
	client.detailURL = server.URL
	if _, err := client.Lookup(context.Background(), "bad-mid"); err == nil {
		t.Fatal("expected error for missing track")
	}
}

func TestCoverFallsBackToSinger(t *testing.T) {
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 2048)...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "T002") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(jpeg)
	}))
	defer server.Close()

	client := NewClient()
	client.httpClient = server.Client()
	client.coverURL = server.URL + "/T002R%dx%dM000%s.jpg"
	client.singerURL = server.URL + "/T001R%dx%dM000%s.jpg"
	data, err := client.Cover(context.Background(), &Track{AlbumMID: "album", SingerMID: "singer"}, 500)
	if err != nil {
		t.Fatalf("Cover: %v", err)
	}
	if len(data) != len(jpeg) {
		t.Fatalf("unexpected cover size: %d", len(data))
	}
}

func TestApplyWritesTagsAndCover(t *testing.T) {
	ffmpeg := os.Getenv("FFMPEG")
	if ffmpeg == "" {
		if found, err := exec.LookPath("ffmpeg"); err == nil {
			ffmpeg = found
		}
	}
	if ffmpeg == "" {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	audioPath := filepath.Join(dir, "in.mp3")
	coverPath := filepath.Join(dir, "cover.jpg")

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(ffmpeg, append([]string{"-y", "-hide_banner", "-loglevel", "error"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg %v: %v (%s)", args, err, out)
		}
	}
	run("-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "libmp3lame", audioPath)
	run("-f", "lavfi", "-i", "color=c=blue:s=300x300", "-frames:v", "1", coverPath)

	cover, err := os.ReadFile(coverPath)
	if err != nil {
		t.Fatal(err)
	}
	track := &Track{
		Title:    "Test Song",
		Artists:  []string{"Test Artist"},
		Album:    "Test Album",
		Track:    4,
		Year:     "2024",
		Genre:    "Pop",
		AlbumMID: "000testalbum00",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := NewClient()
	if err := client.Apply(ctx, ffmpeg, audioPath, track, cover); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	probe := exec.Command(ffmpeg, "-hide_banner", "-i", audioPath)
	output, _ := probe.CombinedOutput()
	text := string(output)
	for _, want := range []string{"attached pic", "title", "Test Song", "album", "Test Album"} {
		if !strings.Contains(text, want) {
			t.Fatalf("ffmpeg output missing %q:\n%s", want, text)
		}
	}
}
