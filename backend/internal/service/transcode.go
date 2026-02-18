package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var validMP3Qualities = map[int]struct{}{0: {}, 2: {}, 5: {}, 7: {}}

func NormalizeOutputFormat(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "mp3", "flac", "wav", "copy":
		return v
	default:
		return "mp3"
	}
}

func NormalizeMP3Quality(raw int) int {
	if _, ok := validMP3Qualities[raw]; ok {
		return raw
	}
	return 2
}

func DetectAudioExt(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	head := make([]byte, 12)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	head = head[:n]

	switch {
	case bytes.HasPrefix(head, []byte("fLaC")):
		return ".flac", nil
	case bytes.HasPrefix(head, []byte("ID3")):
		return ".mp3", nil
	case len(head) >= 2 && head[0] == 0xFF && (head[1]&0xE0) == 0xE0:
		return ".mp3", nil
	case bytes.HasPrefix(head, []byte("RIFF")):
		return ".wav", nil
	case bytes.HasPrefix(head, []byte("OggS")):
		return ".ogg", nil
	default:
		return "", fmt.Errorf("%w: unknown audio header", ErrUnknownAudio)
	}
}

func BuildOutputPath(outputDir, baseName, outputFormat string) string {
	return filepath.Join(outputDir, fmt.Sprintf("%s.%s", baseName, outputFormat))
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func TranscodeToFormat(ctx context.Context, ffmpegBin, inputPath, outputPath, outputFormat string, mp3Quality int) error {
	format := NormalizeOutputFormat(outputFormat)
	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-i", inputPath, "-map_metadata", "0"}

	switch format {
	case "wav":
		args = append(args, "-c:a", "pcm_s16le", outputPath)
	case "flac":
		args = append(args, "-c:a", "flac", outputPath)
	default:
		args = append(args, "-q:a", fmt.Sprintf("%d", NormalizeMP3Quality(mp3Quality)), outputPath)
	}

	cmd := exec.CommandContext(ctx, ffmpegBin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%w: %s", ErrTranscodeProcess, msg)
	}

	if _, err := os.Stat(outputPath); err != nil {
		return fmt.Errorf("%w: ffmpeg output missing", ErrTranscodeProcess)
	}
	return nil
}
