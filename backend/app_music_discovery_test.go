package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindLocalMusicGroupsSupportedFiles(t *testing.T) {
	root := t.TempDir()
	kugouDir := filepath.Join(root, "KuGou")
	qqDir := filepath.Join(root, "Music", "VipSongsDownload")
	mustWriteDiscoveryFile(t, filepath.Join(kugouDir, "top.kgg"), "kgg")
	mustWriteDiscoveryFile(t, filepath.Join(kugouDir, "album", "nested.ncm"), "ncm")
	mustWriteDiscoveryFile(t, filepath.Join(kugouDir, "ignored.mp3"), "mp3")
	mustWriteDiscoveryFile(t, filepath.Join(qqDir, "legacy.qmcflac"), "qmc")
	mustWriteDiscoveryFile(t, filepath.Join(qqDir, "lossless.MFLAC"), "mflac")
	mustWriteDiscoveryFile(t, filepath.Join(qqDir, "high-quality.mgg"), "mgg")
	mustWriteDiscoveryFile(t, filepath.Join(qqDir, "ignored.mp3"), "mp3")

	result, err := findLocalMusic(context.Background(), []musicSearchSource{
		{
			ID:   "kugou",
			Name: "酷狗音乐",
			Directories: []musicSearchDirectory{
				{Path: kugouDir},
				{Path: kugouDir, Recursive: true},
			},
		},
		{
			ID:          "qq",
			Name:        "QQ 音乐",
			Icon:        "i-brand-qq",
			Directories: []musicSearchDirectory{{Path: qqDir, Recursive: true}},
		},
	}, 500)
	if err != nil {
		t.Fatalf("findLocalMusic() error = %v", err)
	}
	if result.Truncated {
		t.Fatal("findLocalMusic() unexpectedly truncated the result")
	}
	if len(result.Groups) != 2 {
		t.Fatalf("group count = %d, want 2", len(result.Groups))
	}
	if got := len(result.Groups[0].Files); got != 2 {
		t.Fatalf("KuGou file count = %d, want 2", got)
	}
	if got := len(result.Groups[1].Files); got != 3 {
		t.Fatalf("QQ Music file count = %d, want 3", got)
	}
	if result.Groups[1].ID != "qq" || result.Groups[1].Name != "QQ 音乐" || result.Groups[1].Icon != "i-brand-qq" {
		t.Fatalf("QQ Music group metadata = %#v", result.Groups[1])
	}
	for _, group := range result.Groups {
		for _, file := range group.Files {
			if strings.EqualFold(filepath.Ext(file.Name), ".mp3") {
				t.Fatalf("unsupported file included: %s", file.Path)
			}
		}
	}
}

func TestFindLocalMusicHonorsFileLimit(t *testing.T) {
	root := t.TempDir()
	mustWriteDiscoveryFile(t, filepath.Join(root, "one.kgm"), "1")
	mustWriteDiscoveryFile(t, filepath.Join(root, "two.kgma"), "2")

	result, err := findLocalMusic(context.Background(), []musicSearchSource{{
		ID:          "kugou",
		Name:        "酷狗音乐",
		Directories: []musicSearchDirectory{{Path: root, Recursive: true}},
	}}, 1)
	if err != nil {
		t.Fatalf("findLocalMusic() error = %v", err)
	}
	if !result.Truncated {
		t.Fatal("findLocalMusic() should report a truncated result")
	}
	if len(result.Groups) != 1 || len(result.Groups[0].Files) != 1 {
		t.Fatalf("limited result = %#v, want one file", result.Groups)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("truncated result should include a warning")
	}
}

func TestFindLocalMusicClassifiesOverlappingDirectoriesByFormat(t *testing.T) {
	root := t.TempDir()
	qqDir := filepath.Join(root, "Music", "VipSongsDownload")
	mustWriteDiscoveryFile(t, filepath.Join(root, "cloud.ncm"), "ncm")
	mustWriteDiscoveryFile(t, filepath.Join(qqDir, "qq-lossless.mflac"), "mflac")
	mustWriteDiscoveryFile(t, filepath.Join(qqDir, "qq-high-quality.mgg"), "mgg")

	result, err := findLocalMusic(context.Background(), []musicSearchSource{
		{
			ID:          "netease",
			Name:        "网易云音乐",
			Directories: []musicSearchDirectory{{Path: root, Recursive: true}},
		},
		{
			ID:          "qq",
			Name:        "QQ 音乐",
			Directories: []musicSearchDirectory{{Path: qqDir, Recursive: true}},
		},
	}, 500)
	if err != nil {
		t.Fatalf("findLocalMusic() error = %v", err)
	}
	if len(result.Groups) != 2 {
		t.Fatalf("groups = %#v, want NetEase and QQ Music", result.Groups)
	}
	if result.Groups[0].ID != "netease" || len(result.Groups[0].Files) != 1 || !strings.EqualFold(filepath.Ext(result.Groups[0].Files[0].Name), ".ncm") {
		t.Fatalf("NetEase group = %#v, want only NCM", result.Groups[0])
	}
	if result.Groups[1].ID != "qq" || len(result.Groups[1].Files) != 2 {
		t.Fatalf("QQ Music group = %#v, want MFLAC and MGG", result.Groups[1])
	}
	for _, file := range result.Groups[1].Files {
		if ext := strings.ToLower(filepath.Ext(file.Name)); ext != ".mflac" && ext != ".mgg" {
			t.Fatalf("QQ Music group contains %q", file.Name)
		}
	}
}

func TestNormalizeMusicSearchDirectories(t *testing.T) {
	root := t.TempDir()
	directories := normalizeMusicSearchDirectories([]musicSearchDirectory{
		{Path: "relative", Recursive: true},
		{Path: `\\server\music`, Recursive: true},
		{Path: root},
		{Path: root, Recursive: true},
	})
	if len(directories) != 1 {
		t.Fatalf("directory count = %d, want 1", len(directories))
	}
	if !directories[0].Recursive {
		t.Fatal("duplicate recursive directory should upgrade the existing entry")
	}
}

func mustWriteDiscoveryFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
