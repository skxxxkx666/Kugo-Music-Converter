package handler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"kugo-music-converter/internal/config"
	"kugo-music-converter/internal/service"
)

func TestBuildLocalConvertRequest(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "song.ncm")
	if err := os.WriteFile(inputPath, []byte("fixture"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.MaxFiles = 5
	cfg.MaxFileSize = 1024
	cfg.Concurrency = 3
	h := &ConvertHandler{cfg: cfg}

	req, err := h.buildLocalConvertRequest(LocalConversionRequest{
		Paths:        []string{inputPath, inputPath},
		OutputDir:    filepath.Join(root, "converted"),
		OutputFormat: "FLAC",
		MP3Quality:   99,
		Concurrency:  99,
	})
	if err != nil {
		t.Fatalf("buildLocalConvertRequest() error = %v", err)
	}
	if len(req.Items) != 1 {
		t.Fatalf("items = %d, want 1 unique item", len(req.Items))
	}
	if req.Items[0].Current != 1 || req.Items[0].OriginPath != filepath.Clean(inputPath) {
		t.Fatalf("unexpected batch item: %#v", req.Items[0])
	}
	if req.OutputFormat != "flac" {
		t.Fatalf("output format = %q, want flac", req.OutputFormat)
	}
	if req.MP3Quality != 2 {
		t.Fatalf("mp3 quality = %d, want normalized default 2", req.MP3Quality)
	}
	if req.Concurrency != runtimeMaxConcurrency() {
		t.Fatalf("concurrency = %d, want runtime cap %d", req.Concurrency, runtimeMaxConcurrency())
	}
	if info, statErr := os.Stat(req.OutputDir); statErr != nil || !info.IsDir() {
		t.Fatalf("output directory was not created: %v", statErr)
	}
}

func TestBuildLocalConvertRequestRejectsInvalidInputs(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.MaxFiles = 1
	cfg.MaxFileSize = 1
	h := &ConvertHandler{cfg: cfg}

	unsupported := filepath.Join(root, "song.txt")
	if err := os.WriteFile(unsupported, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	tooLarge := filepath.Join(root, "large.ncm")
	if err := os.WriteFile(tooLarge, []byte("xx"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	valid := filepath.Join(root, "valid.ncm")
	if err := os.WriteFile(valid, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tests := []struct {
		name  string
		input LocalConversionRequest
		code  string
	}{
		{name: "empty", input: LocalConversionRequest{OutputDir: root}, code: ErrNoFiles},
		{name: "too-many", input: LocalConversionRequest{Paths: []string{unsupported, tooLarge}, OutputDir: root}, code: ErrTooManyFiles},
		{name: "missing", input: LocalConversionRequest{Paths: []string{filepath.Join(root, "missing.ncm")}, OutputDir: root}, code: ErrInputPathDenied},
		{name: "unsupported", input: LocalConversionRequest{Paths: []string{unsupported}, OutputDir: root}, code: ErrUnsupportedFormat},
		{name: "too-large", input: LocalConversionRequest{Paths: []string{tooLarge}, OutputDir: root}, code: ErrFileTooLarge},
		{name: "output-required", input: LocalConversionRequest{Paths: []string{valid}}, code: ErrOutputRequired},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := h.buildLocalConvertRequest(test.input)
			if err == nil {
				t.Fatal("buildLocalConvertRequest() error = nil")
			}
			var appErr *AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("error type = %T, want *AppError", err)
			}
			if appErr.Code != test.code {
				t.Fatalf("error code = %q, want %q", appErr.Code, test.code)
			}
		})
	}
}

func TestValidateLocalConversionRequiresRuntime(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "song.ncm")
	if err := os.WriteFile(inputPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	h := &ConvertHandler{
		cfg:        config.DefaultConfig(),
		ffmpegPath: filepath.Join(root, "missing-ffmpeg.exe"),
	}
	err := h.ValidateLocalConversion(LocalConversionRequest{Paths: []string{inputPath}, OutputDir: root})
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Code != ErrRuntimeMissing {
		t.Fatalf("error = %#v, want %s", err, ErrRuntimeMissing)
	}
}

func TestConvertLocalPathsRealFile(t *testing.T) {
	inputPath := os.Getenv("KUGO_TEST_INPUT")
	ffmpegPath := os.Getenv("KUGO_TEST_FFMPEG")
	if inputPath == "" || ffmpegPath == "" {
		t.Skip("set KUGO_TEST_INPUT and KUGO_TEST_FFMPEG to run the real-file integration test")
	}

	outputDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.FFmpegBin = ffmpegPath
	cfg.DefaultOutput = outputDir
	cfg.Concurrency = 1
	h := NewConvertHandler(cfg, "test")
	outputFormat := strings.TrimSpace(os.Getenv("KUGO_TEST_OUTPUT_FORMAT"))
	if outputFormat == "" {
		outputFormat = "copy"
	}

	progressEvents := 0
	fileDoneEvents := 0
	summary, err := h.ConvertLocalPaths(context.Background(), LocalConversionRequest{
		Paths:        []string{inputPath},
		OutputDir:    outputDir,
		OutputFormat: outputFormat,
		MP3Quality:   2,
		Concurrency:  1,
	}, func(name string, _ any) {
		switch name {
		case "progress":
			progressEvents++
		case "file-done":
			fileDoneEvents++
		}
	})
	if err != nil {
		t.Fatalf("ConvertLocalPaths() error = %v", err)
	}
	if summary.Success != 1 || summary.Failed != 0 || summary.Cancelled {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if len(summary.Results) != 1 || summary.Results[0].Output == "" {
		t.Fatalf("missing conversion output: %#v", summary.Results)
	}
	if info, statErr := os.Stat(summary.Results[0].Output); statErr != nil || info.Size() == 0 {
		t.Fatalf("converted output is missing or empty: %v", statErr)
	}
	detectedExt, detectErr := service.DetectAudioExt(summary.Results[0].Output)
	if detectErr != nil {
		t.Fatalf("converted output has an invalid audio header: %v", detectErr)
	}
	if outputFormat != "copy" && detectedExt != "."+outputFormat {
		t.Fatalf("converted output extension = %s, want .%s", detectedExt, outputFormat)
	}
	if progressEvents == 0 || fileDoneEvents != 1 {
		t.Fatalf("events: progress=%d fileDone=%d", progressEvents, fileDoneEvents)
	}
}

func TestConvertLocalPathsRealFolder(t *testing.T) {
	inputDir := strings.TrimSpace(os.Getenv("KUGO_TEST_INPUT_DIR"))
	ffmpegPath := strings.TrimSpace(os.Getenv("KUGO_TEST_FFMPEG"))
	if inputDir == "" || ffmpegPath == "" {
		t.Skip("set KUGO_TEST_INPUT_DIR and KUGO_TEST_FFMPEG to run the real-folder integration test")
	}

	paths := make([]string, 0, 64)
	var totalInputBytes int64
	err := filepath.WalkDir(inputDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() || !containsInputExt(entry.Name()) {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		paths = append(paths, path)
		totalInputBytes += info.Size()
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir() error = %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no supported files found in KUGO_TEST_INPUT_DIR")
	}
	sort.Strings(paths)

	outputFormat := strings.TrimSpace(os.Getenv("KUGO_TEST_OUTPUT_FORMAT"))
	if outputFormat == "" {
		outputFormat = "copy"
	}
	concurrency := 1
	if raw := strings.TrimSpace(os.Getenv("KUGO_TEST_CONCURRENCY")); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 {
			concurrency = parsed
		}
	}

	outputDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.FFmpegBin = ffmpegPath
	cfg.DefaultOutput = outputDir
	cfg.Concurrency = concurrency
	h := NewConvertHandler(cfg, "test")
	effectiveConcurrency := normalizeConcurrency(concurrency, cfg.Concurrency)

	started := time.Now()
	summary, err := h.ConvertLocalPaths(context.Background(), LocalConversionRequest{
		Paths:        paths,
		OutputDir:    outputDir,
		DBPath:       strings.TrimSpace(os.Getenv("KUGO_TEST_DB")),
		OutputFormat: outputFormat,
		MP3Quality:   2,
		Concurrency:  concurrency,
	}, nil)
	wallTime := time.Since(started)
	if err != nil {
		t.Fatalf("ConvertLocalPaths() error = %v", err)
	}
	t.Logf(
		"files=%d inputBytes=%d format=%s requestedConcurrency=%d effectiveConcurrency=%d durationMs=%d wall=%s success=%d failed=%d",
		len(paths), totalInputBytes, outputFormat, concurrency, effectiveConcurrency, summary.DurationMs, wallTime.Round(time.Millisecond), summary.Success, summary.Failed,
	)
	if summary.Success != len(paths) || summary.Failed != 0 || summary.Cancelled {
		for _, result := range summary.Results {
			if result.Error != nil {
				t.Logf("failed file=%s code=%s detail=%s", result.File, result.Error.Code, result.Error.Detail)
			}
		}
		t.Fatalf("unexpected summary: success=%d failed=%d cancelled=%v", summary.Success, summary.Failed, summary.Cancelled)
	}
}
