package service

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDetectAudioExtFromReaderPreservesData(t *testing.T) {
	cases := []struct {
		name string
		head []byte
		want string
	}{
		{name: "flac", head: []byte("fLaC"), want: ".flac"},
		{name: "mp3-id3", head: []byte("ID3"), want: ".mp3"},
		{name: "mp3-frame", head: []byte{0xFF, 0xFB}, want: ".mp3"},
		{name: "aac-adts", head: []byte{0xFF, 0xF1, 0x50, 0x80}, want: ".aac"},
		{name: "m4a-ftyp", head: []byte{0x00, 0x00, 0x00, 0x20, 'f', 't', 'y', 'p', 'M', '4', 'A', ' '}, want: ".m4a"},
		{name: "wma-asf", head: []byte{0x30, 0x26, 0xB2, 0x75}, want: ".wma"},
		{name: "dff", head: []byte("FRM8"), want: ".dff"},
		{name: "wav", head: []byte("RIFF"), want: ".wav"},
		{name: "ogg", head: []byte("OggS"), want: ".ogg"},
	}

	for _, tc := range cases {
		src := append(append([]byte(nil), tc.head...), bytes.Repeat([]byte{0x7A}, 32)...)
		ext, wrapped, err := DetectAudioExtFromReader(bytes.NewReader(src))
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if ext != tc.want {
			t.Fatalf("%s: ext = %s, want %s", tc.name, ext, tc.want)
		}
		got, err := io.ReadAll(wrapped)
		if err != nil {
			t.Fatalf("%s: read wrapped failed: %v", tc.name, err)
		}
		if !bytes.Equal(got, src) {
			t.Fatalf("%s: wrapped stream content mismatch", tc.name)
		}
	}
}

func TestAudioExtToFFmpegFormat(t *testing.T) {
	if got := AudioExtToFFmpegFormat(".mp3"); got != "mp3" {
		t.Fatalf("mp3 format mismatch: %s", got)
	}
	if got := AudioExtToFFmpegFormat(".flac"); got != "flac" {
		t.Fatalf("flac format mismatch: %s", got)
	}
	if got := AudioExtToFFmpegFormat(".wav"); got != "wav" {
		t.Fatalf("wav format mismatch: %s", got)
	}
	if got := AudioExtToFFmpegFormat(".ogg"); got != "ogg" {
		t.Fatalf("ogg format mismatch: %s", got)
	}
	if got := AudioExtToFFmpegFormat(".aac"); got != "aac" {
		t.Fatalf("aac format mismatch: %s", got)
	}
	if got := AudioExtToFFmpegFormat(".m4a"); got != "mp4" {
		t.Fatalf("m4a format mismatch: %s", got)
	}
	if got := AudioExtToFFmpegFormat(".wma"); got != "asf" {
		t.Fatalf("wma format mismatch: %s", got)
	}
	if got := AudioExtToFFmpegFormat(".dff"); got != "dff" {
		t.Fatalf("dff format mismatch: %s", got)
	}
	if got := AudioExtToFFmpegFormat(".unknown"); got != "" {
		t.Fatalf("unknown ext should map to empty, got: %s", got)
	}
}

func TestDetectAudioExtFromReaderUnknownHeaderHex(t *testing.T) {
	_, _, err := DetectAudioExtFromReader(bytes.NewReader([]byte{0x00, 0x11, 0x22, 0x33}))
	if err == nil {
		t.Fatal("expected error for unknown header, got nil")
	}
	message := err.Error()
	if !strings.Contains(message, "unknown audio header [hex: 00 11 22 33]") {
		t.Fatalf("unexpected error message: %s", message)
	}
}

func TestIsOGGCRCError(t *testing.T) {
	if !IsOGGCRCError("[ogg] CRC mismatch!") {
		t.Fatal("expected crc mismatch to be recognized")
	}
	if !IsOGGCRCError("Header processing failed") {
		t.Fatal("expected header processing failed to be recognized")
	}
	if IsOGGCRCError("invalid data found when processing input") {
		t.Fatal("did not expect generic error to be recognized as ogg crc")
	}
}

func TestAppendFFmpegOutputArgsUsesExplicitEncoders(t *testing.T) {
	tests := []struct {
		format string
		want   []string
	}{
		{format: "mp3", want: []string{"-c:a", "libmp3lame", "-q:a", "2", "out.mp3"}},
		{format: "flac", want: []string{"-c:a", "flac", "out.mp3"}},
		{format: "wav", want: []string{"-c:a", "pcm_s16le", "out.mp3"}},
	}
	for _, tc := range tests {
		got := appendFFmpegOutputArgs(nil, "out.mp3", tc.format, 2)
		if !slices.Equal(got, tc.want) {
			t.Errorf("format %s args = %v, want %v", tc.format, got, tc.want)
		}
	}
}

func TestCopyReaderToFile(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "out.bin")
	src := bytes.NewReader(bytes.Repeat([]byte{0x3C}, 8192))

	if err := CopyReaderToFile(src, dst); err != nil {
		t.Fatalf("CopyReaderToFile failed: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(data) != 8192 {
		t.Fatalf("copied size mismatch: %d", len(data))
	}
	for _, b := range data {
		if b != 0x3C {
			t.Fatalf("unexpected content byte: %x", b)
		}
	}
}
