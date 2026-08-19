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
	qqDir := filepath.Join(root, "QQMusic")
	mustWriteDiscoveryFile(t, filepath.Join(kugouDir, "top.kgg"), "kgg")
	mustWriteDiscoveryFile(t, filepath.Join(kugouDir, "album", "nested.ncm"), "ncm")
	mustWriteDiscoveryFile(t, filepath.Join(kugouDir, "ignored.mp3"), "mp3")
	mustWriteDiscoveryFile(t, filepath.Join(qqDir, "track.qmcflac"), "qmc")

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
	if got := len(result.Groups[1].Files); got != 1 {
		t.Fatalf("QQ Music file count = %d, want 1", got)
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
