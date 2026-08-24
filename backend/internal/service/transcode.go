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

	"kugo-music-converter/internal/utils"
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
	case len(head) >= 2 && head[0] == 0xFF && (head[1] == 0xF1 || head[1] == 0xF9):
		return ".aac", nil
	case len(head) >= 2 && head[0] == 0xFF && (head[1]&0xE0) == 0xE0:
		return ".mp3", nil
	case len(head) >= 8 && bytes.Equal(head[4:8], []byte("ftyp")):
		return ".m4a", nil
	case bytes.HasPrefix(head, []byte{0x30, 0x26, 0xB2, 0x75}):
		return ".wma", nil
	case bytes.HasPrefix(head, []byte("FRM8")):
		return ".dff", nil
	case bytes.HasPrefix(head, []byte("RIFF")):
		return ".wav", nil
	case bytes.HasPrefix(head, []byte("OggS")):
		return ".ogg", nil
	default:
		return "", fmt.Errorf("%w: unknown audio header [hex: %s]", ErrUnknownAudio, formatHeaderHex(head, 16))
	}
}

func formatHeaderHex(head []byte, limit int) string {
	if len(head) == 0 {
		return "empty"
	}
	if limit <= 0 || limit > len(head) {
		limit = len(head)
	}
	var builder strings.Builder
	for i := 0; i < limit; i++ {
		if i > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(fmt.Sprintf("%02X", head[i]))
	}
	return builder.String()
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
	case ".aac":
		return "aac"
	case ".m4a":
		return "mp4"
	case ".wma":
		return "asf"
	case ".dff":
		return "dff"
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

func appendFFmpegOutputArgs(args []string, outputPath, outputFormat string, mp3Quality int) []string {
	format := NormalizeOutputFormat(outputFormat)
	switch format {
	case "wav":
		return append(args, "-c:a", "pcm_s16le", outputPath)
	case "flac":
		return append(args, "-c:a", "flac", outputPath)
	default:
		return append(args, "-c:a", "libmp3lame", "-q:a", fmt.Sprintf("%d", NormalizeMP3Quality(mp3Quality)), outputPath)
	}
}

func buildFFmpegArgsForReader(inputFormat, outputPath, outputFormat string, mp3Quality int, tolerant bool) []string {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-threads", "1",
	}
	if tolerant {
		args = append(args, "-err_detect", "ignore_err", "-fflags", "+discardcorrupt+genpts")
	}
	args = append(args, "-f", inputFormat, "-i", "pipe:0", "-map_metadata", "0")
	return appendFFmpegOutputArgs(args, outputPath, outputFormat, mp3Quality)
}

func buildFFmpegArgsForFile(inputPath, outputPath, outputFormat string, mp3Quality int, tolerant bool) []string {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-threads", "1",
	}
	if tolerant {
		args = append(args, "-err_detect", "ignore_err", "-fflags", "+discardcorrupt+genpts")
	}
	args = append(args, "-i", inputPath, "-map_metadata", "0")
	return appendFFmpegOutputArgs(args, outputPath, outputFormat, mp3Quality)
}

func runFFmpeg(ctx context.Context, ffmpegBin string, args []string, input io.Reader) (string, error) {
	cmd := exec.CommandContext(ctx, ffmpegBin, args...)
	utils.ConfigureBackgroundCommand(cmd)
	cmd.Stdin = input
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return msg, fmt.Errorf("%w: %s", ErrTranscodeProcess, msg)
	}
	return strings.TrimSpace(stderr.String()), nil
}

func transcodeReaderOnce(ctx context.Context, ffmpegBin string, input io.Reader, inputFormat, outputPath, outputFormat string, mp3Quality int, tolerant bool) (string, error) {
	if strings.TrimSpace(inputFormat) == "" {
		return "", fmt.Errorf("%w: missing input format for pipe transcode", ErrTranscodeProcess)
	}
	stderr, err := runFFmpeg(ctx, ffmpegBin, buildFFmpegArgsForReader(inputFormat, outputPath, outputFormat, mp3Quality, tolerant), input)
	if err != nil {
		return stderr, err
	}
	if _, err := os.Stat(outputPath); err != nil {
		return stderr, fmt.Errorf("%w: ffmpeg output missing", ErrTranscodeProcess)
	}
	return stderr, nil
}

func TranscodeReaderToFormat(ctx context.Context, ffmpegBin string, input io.Reader, inputFormat, outputPath, outputFormat string, mp3Quality int) error {
	_, err := transcodeReaderOnce(ctx, ffmpegBin, input, inputFormat, outputPath, outputFormat, mp3Quality, false)
	return err
}

func transcodeFileOnce(ctx context.Context, ffmpegBin, inputPath, outputPath, outputFormat string, mp3Quality int, tolerant bool) (string, error) {
	stderr, err := runFFmpeg(ctx, ffmpegBin, buildFFmpegArgsForFile(inputPath, outputPath, outputFormat, mp3Quality, tolerant), nil)
	if err != nil {
		return stderr, err
	}
	if _, err := os.Stat(outputPath); err != nil {
		return stderr, fmt.Errorf("%w: ffmpeg output missing", ErrTranscodeProcess)
	}
	return stderr, nil
}

func ValidateAudioFileStrict(ctx context.Context, ffmpegBin, inputPath string) error {
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-xerror",
		"-nostdin",
		"-threads", "1",
		"-i", inputPath,
		"-map", "0:a:0",
		"-f", "null",
		"-",
	}
	_, err := runFFmpeg(ctx, ffmpegBin, args, nil)
	return err
}

func RepairFLACFile(ctx context.Context, ffmpegBin, inputPath, outputPath string) error {
	_, err := transcodeFileOnce(ctx, ffmpegBin, inputPath, outputPath, "flac", 0, true)
	return err
}

func TranscodeToFormat(ctx context.Context, ffmpegBin, inputPath, outputPath, outputFormat string, mp3Quality int) error {
	_, err := transcodeFileOnce(ctx, ffmpegBin, inputPath, outputPath, outputFormat, mp3Quality, false)
	return err
}

func IsOGGCRCError(stderr string) bool {
	msg := strings.ToLower(strings.TrimSpace(stderr))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "crc mismatch") || strings.Contains(msg, "header processing failed")
}

func TranscodeToFormatWithRetry(ctx context.Context, ffmpegBin, inputPath, outputPath, outputFormat string, mp3Quality int) (bool, error) {
	stderr, err := transcodeFileOnce(ctx, ffmpegBin, inputPath, outputPath, outputFormat, mp3Quality, false)
	if err == nil {
		return false, nil
	}
	if !IsOGGCRCError(stderr) {
		return false, err
	}
	_, retryErr := transcodeFileOnce(ctx, ffmpegBin, inputPath, outputPath, outputFormat, mp3Quality, true)
	if retryErr != nil {
		return true, retryErr
	}
	return true, nil
}
