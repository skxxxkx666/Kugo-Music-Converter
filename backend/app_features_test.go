package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"kugo-music-converter/internal/service"
)

func TestMessageDialogConfirmed(t *testing.T) {
	tests := []struct {
		name             string
		result           string
		affirmativeLabel string
		want             bool
	}{
		{name: "windows yes", result: "Yes", affirmativeLabel: "删除", want: true},
		{name: "windows yes case insensitive", result: " yes ", affirmativeLabel: "删除", want: true},
		{name: "custom history label", result: "删除", affirmativeLabel: "删除", want: true},
		{name: "custom cancellation label", result: "取消任务", affirmativeLabel: "取消任务", want: true},
		{name: "windows no", result: "No", affirmativeLabel: "删除", want: false},
		{name: "custom cancel label", result: "取消", affirmativeLabel: "删除", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := messageDialogConfirmed(tc.result, tc.affirmativeLabel); got != tc.want {
				t.Fatalf("messageDialogConfirmed(%q, %q) = %v, want %v", tc.result, tc.affirmativeLabel, got, tc.want)
			}
		})
	}
}

func TestScanAudioDirectoryFiltersSupportedFiles(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "root.ncm"), []byte("root"))
	writeTestFile(t, filepath.Join(root, "ignore.mp3"), []byte("plain"))
	writeTestFile(t, filepath.Join(nested, "nested.kgg"), []byte("nested"))

	app := NewApp("test")
	app.ctx = context.Background()

	flat, err := app.ScanAudioDirectory(ScanDirectoryRequest{Path: root})
	if err != nil {
		t.Fatalf("ScanAudioDirectory(flat) error = %v", err)
	}
	if len(flat.Files) != 1 || flat.Files[0].Name != "root.ncm" {
		t.Fatalf("flat files = %+v", flat.Files)
	}

	recursive, err := app.ScanAudioDirectory(ScanDirectoryRequest{Path: root, Recursive: true})
	if err != nil {
		t.Fatalf("ScanAudioDirectory(recursive) error = %v", err)
	}
	if len(recursive.Files) != 2 {
		t.Fatalf("recursive file count = %d, want 2", len(recursive.Files))
	}
	if recursive.TotalSize != int64(len("root")+len("nested")) {
		t.Fatalf("recursive total size = %d", recursive.TotalSize)
	}
}

func TestPreviewRequiresRegisteredConversionOutput(t *testing.T) {
	root := t.TempDir()
	registered := filepath.Join(root, "registered.mp3")
	unregistered := filepath.Join(root, "unregistered.mp3")
	writeTestFile(t, registered, []byte("0123456789"))
	writeTestFile(t, unregistered, []byte("abcdefghij"))

	app := NewApp("test")
	if _, err := app.GetPreviewURL(unregistered); err == nil {
		t.Fatal("GetPreviewURL(unregistered) error = nil")
	}

	app.registerPreviewResult(service.BatchFileDoneEvent{Status: "ok", Output: registered})
	previewURL, err := app.GetPreviewURL(registered)
	if err != nil {
		t.Fatalf("GetPreviewURL(registered) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, previewURL, nil)
	req.Header.Set("Range", "bytes=0-3")
	recorder := httptest.NewRecorder()
	app.assetHandler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusPartialContent {
		t.Fatalf("preview status = %d, want %d", recorder.Code, http.StatusPartialContent)
	}
	if got := recorder.Body.String(); got != "0123" {
		t.Fatalf("preview body = %q", got)
	}

	missingRecorder := httptest.NewRecorder()
	app.assetHandler().ServeHTTP(missingRecorder, httptest.NewRequest(http.MethodGet, desktopPreviewPrefix+"missing", nil))
	if missingRecorder.Code != http.StatusNotFound {
		t.Fatalf("missing preview status = %d", missingRecorder.Code)
	}
}

func TestResolveDroppedPathsAcceptsFilesAndDirectories(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	direct := filepath.Join(root, "direct.ncm")
	writeTestFile(t, direct, []byte("direct"))
	writeTestFile(t, filepath.Join(nested, "nested.kgm"), []byte("nested"))
	unsupported := filepath.Join(root, "ignore.txt")
	writeTestFile(t, unsupported, []byte("ignore"))

	app := NewApp("test")
	app.ctx = context.Background()
	result, err := app.ResolveDroppedPaths(DroppedPathsRequest{
		Paths:     []string{direct, nested, unsupported},
		Recursive: true,
	})
	if err != nil {
		t.Fatalf("ResolveDroppedPaths() error = %v", err)
	}
	if len(result.Files) != 2 {
		t.Fatalf("dropped files = %+v", result.Files)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("warnings = %+v", result.Warnings)
	}
}

func TestScanDirectoriesAppliesCustomFilter(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "song.mp3"), []byte("mp3"))
	writeTestFile(t, filepath.Join(root, "song.ncm"), []byte("ncm"))

	app := NewApp("test")
	app.ctx = context.Background()
	result, err := app.ScanDirectories(ScanDirectoriesRequest{
		Paths:  []string{root},
		Filter: ".mp3",
	})
	if err != nil {
		t.Fatalf("ScanDirectories() error = %v", err)
	}
	if result.TotalFiles != 1 || len(result.Folders) != 1 || result.Folders[0].Files[0].Name != "song.mp3" {
		t.Fatalf("scan result = %+v", result)
	}
}

func TestNormalizeTextExportRequest(t *testing.T) {
	filename, extension, err := normalizeTextExportRequest(SaveTextFileRequest{
		DefaultFilename: `C:\Temp\failed.exe`,
		Content:         "row",
		Extension:       ".csv",
	})
	if err != nil {
		t.Fatalf("normalizeTextExportRequest() error = %v", err)
	}
	if filename != "failed.csv" || extension != ".csv" {
		t.Fatalf("normalized = %q, %q", filename, extension)
	}
	if _, _, err := normalizeTextExportRequest(SaveTextFileRequest{Extension: ".exe"}); err == nil {
		t.Fatal("unsupported export extension was accepted")
	}
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}
