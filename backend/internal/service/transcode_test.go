package service

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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
	if got := AudioExtToFFmpegFormat(".aac"); got != "" {
		t.Fatalf("unknown ext should map to empty, got: %s", got)
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
