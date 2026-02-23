package service

import (
	"bufio"
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

const (
	copyBufferSize = 128 * 1024
	writeBufferMB  = 256 * 1024
)

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

func detectAudioExtByHeader(head []byte) (string, error) {
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
	return detectAudioExtByHeader(head[:n])
}

func DetectAudioExtFromReader(src io.Reader) (string, io.Reader, error) {
	head := make([]byte, 12)
	n, err := io.ReadFull(src, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", nil, err
	}
	head = head[:n]
	ext, err := detectAudioExtByHeader(head)
	if err != nil {
		return "", nil, err
	}
	return ext, io.MultiReader(bytes.NewReader(head), src), nil
}

func AudioExtToFFmpegFormat(ext string) string {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".flac":
		return "flac"
	case ".mp3":
		return "mp3"
	case ".wav":
		return "wav"
	case ".ogg":
		return "ogg"
	default:
		return ""
	}
}

func BuildOutputPath(outputDir, baseName, outputFormat string) string {
	return filepath.Join(outputDir, fmt.Sprintf("%s.%s", baseName, outputFormat))
}

func CopyReaderToFile(src io.Reader, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	writer := bufio.NewWriterSize(out, writeBufferMB)
	buf := make([]byte, copyBufferSize)
	if _, err := io.CopyBuffer(writer, src, buf); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	return out.Sync()
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	return CopyReaderToFile(in, dst)
}

func buildFFmpegArgsForReader(inputFormat, outputPath, outputFormat string, mp3Quality int) []string {
	format := NormalizeOutputFormat(outputFormat)
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-threads", "1",
		"-f", inputFormat,
		"-i", "pipe:0",
		"-map_metadata", "0",
	}

	switch format {
	case "wav":
		return append(args, "-c:a", "pcm_s16le", outputPath)
	case "flac":
		return append(args, "-c:a", "flac", outputPath)
	default:
		return append(args, "-q:a", fmt.Sprintf("%d", NormalizeMP3Quality(mp3Quality)), outputPath)
	}
}

func TranscodeReaderToFormat(ctx context.Context, ffmpegBin string, input io.Reader, inputFormat, outputPath, outputFormat string, mp3Quality int) error {
	if strings.TrimSpace(inputFormat) == "" {
		return fmt.Errorf("%w: missing input format for pipe transcode", ErrTranscodeProcess)
	}
	cmd := exec.CommandContext(ctx, ffmpegBin, buildFFmpegArgsForReader(inputFormat, outputPath, outputFormat, mp3Quality)...)
	cmd.Stdin = input
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

func TranscodeToFormat(ctx context.Context, ffmpegBin, inputPath, outputPath, outputFormat string, mp3Quality int) error {
	format := NormalizeOutputFormat(outputFormat)
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-threads", "1",
		"-i", inputPath,
		"-map_metadata", "0",
	}

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
