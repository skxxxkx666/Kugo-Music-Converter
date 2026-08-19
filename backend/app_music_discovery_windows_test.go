//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestFindLocalMusicLive(t *testing.T) {
	if os.Getenv("KUGO_TEST_LOCAL_MUSIC") != "1" {
		t.Skip("set KUGO_TEST_LOCAL_MUSIC=1 to scan configured local music folders")
	}
	app := NewApp("test")
	app.ctx = context.Background()
	result, err := app.FindLocalMusic()
	if err != nil {
		t.Fatalf("FindLocalMusic() error = %v", err)
	}
	total := 0
	for _, group := range result.Groups {
		t.Logf("%s: %d files", group.Name, len(group.Files))
		total += len(group.Files)
	}
	if total == 0 {
		t.Fatal("FindLocalMusic() found no supported files in configured local music folders")
	}

	expectedDir := strings.TrimSpace(os.Getenv("KUGO_TEST_EXPECTED_MUSIC_DIR"))
	if expectedDir == "" {
		return
	}
	foundPaths := make(map[string]struct{}, total)
	for _, group := range result.Groups {
		for _, file := range group.Files {
			foundPaths[normalizeDesktopPath(file.Path)] = struct{}{}
		}
	}
	expected := 0
	missed := make([]string, 0)
	if err := filepath.WalkDir(expectedDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		if _, supported := supportedDesktopExts[strings.ToLower(filepath.Ext(entry.Name()))]; !supported {
			return nil
		}
		expected++
		if _, found := foundPaths[normalizeDesktopPath(path)]; !found {
			missed = append(missed, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("WalkDir(%q) error = %v", expectedDir, err)
	}
	t.Logf("expected=%d discovered=%d missed=%d", expected, expected-len(missed), len(missed))
	if len(missed) > 0 {
		t.Fatalf("FindLocalMusic() missed configured samples: %v", missed)
	}
}

func TestReadMusicINIValueFromUTF16LE(t *testing.T) {
	path := filepath.Join(t.TempDir(), "KuGou.ini")
	content := "[Other]\r\nDownloadPath=C:\\Wrong\r\n[DownloadConfigSection]\r\nDownloadPath=E:\\KuGou\\\r\n"
	units := utf16.Encode([]rune(content))
	data := []byte{0xff, 0xfe}
	for _, unit := range units {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if got := readMusicINIValue(path, "DownloadConfigSection", "DownloadPath"); got != `E:\KuGou\` {
		t.Fatalf("DownloadPath = %q, want %q", got, `E:\KuGou\`)
	}
}

func TestExpandWindowsEnvironment(t *testing.T) {
	t.Setenv("KUGO_MUSIC_TEST_ROOT", `D:\Media`)
	if got := expandWindowsEnvironment(`%KUGO_MUSIC_TEST_ROOT%\Library`); got != `D:\Media\Library` {
		t.Fatalf("expanded path = %q, want %q", got, `D:\Media\Library`)
	}
}

func TestExtractWindowsAbsolutePathsFromBinaryAndUTF16(t *testing.T) {
	plainPath := `E:\CloudMusic\VipSongsDownload\song.ncm`
	utf16Path := `D:\QQMusic\custom`
	units := utf16.Encode([]rune(utf16Path))
	data := append([]byte("\x00prefix\x01"+plainPath+"\x00"), 0xff, 0xfe)
	for _, unit := range units {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}

	got := extractWindowsAbsolutePaths(data)
	joined := strings.Join(got, "\n")
	for _, expected := range []string{plainPath, utf16Path} {
		if !strings.Contains(strings.ToLower(joined), strings.ToLower(expected)) {
			t.Fatalf("paths = %v, want %q", got, expected)
		}
	}
}

func TestMusicDirectoriesFromConfigRoots(t *testing.T) {
	root := t.TempDir()
	downloadDirectory := filepath.Join(root, "CustomDownloads")
	if err := os.MkdirAll(downloadDirectory, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	configPath := filepath.Join(root, "library.dat")
	if err := os.WriteFile(configPath, []byte("\x01"+downloadDirectory+"\x00"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got := musicDirectoriesFromConfigRoots([]string{root})
	if len(got) != 1 || !strings.EqualFold(got[0].Path, downloadDirectory) || !got[0].Recursive {
		t.Fatalf("directories = %#v, want recursive %q", got, downloadDirectory)
	}
}
