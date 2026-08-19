package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"unlock-music.dev/cli/algo/qmc"
)

func TestRecoverDecryptPanic(t *testing.T) {
	tmp, err := os.CreateTemp("", "decrypt_panic_*.tmp")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	path := tmp.Name()
	_ = tmp.Close()

	called := false
	cleanup := func() { called = true }
	outPath := path
	var decryptErr error

	func() {
		defer recoverDecryptPanic("kgg", &outPath, &cleanup, &decryptErr)
		panic("boom")
	}()

	if outPath != "" {
		t.Fatalf("outPath = %q, want empty", outPath)
	}
	if decryptErr == nil {
		t.Fatal("expected decrypt error after panic")
	}
	if !errors.Is(decryptErr, ErrDecryptProcess) {
		t.Fatalf("error = %v, want wrapped ErrDecryptProcess", decryptErr)
	}
	if !strings.Contains(decryptErr.Error(), "kgg decoder panic") {
		t.Fatalf("error = %v, want panic message", decryptErr)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected temp file removed, stat error = %v", statErr)
	}

	cleanup()
	if called {
		t.Fatal("cleanup should be replaced with noop after panic")
	}
}

func TestRecoverDecryptPanicWithoutPanic(t *testing.T) {
	called := false
	cleanup := func() { called = true }
	outPath := "keep"
	var decryptErr error

	func() {
		defer recoverDecryptPanic("kgm", &outPath, &cleanup, &decryptErr)
	}()

	if outPath != "keep" {
		t.Fatalf("outPath = %q, want keep", outPath)
	}
	if decryptErr != nil {
		t.Fatalf("unexpected error = %v", decryptErr)
	}

	cleanup()
	if !called {
		t.Fatal("cleanup should remain unchanged when no panic")
	}
}

func TestDecryptKwmPureGo(t *testing.T) {
	plain := append([]byte("ID3"), bytes.Repeat([]byte{0x5a}, 125)...)
	path := filepath.Join(t.TempDir(), "sample.kwm")
	if err := os.WriteFile(path, buildKwmFixture(plain), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	reader, err := NewDecryptService(nil).DecryptFileByExt(path)
	if err != nil {
		t.Fatalf("DecryptFileByExt() error = %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("KWM decrypted bytes do not match the fixture")
	}
}

func TestDecryptLegacyQmcPureGo(t *testing.T) {
	plain := append([]byte("ID3"), bytes.Repeat([]byte{0x31}, 70*1024)...)
	encrypted := append([]byte(nil), plain...)
	cipher, err := qmc.NewQmcCipherDecoder(nil)
	if err != nil {
		t.Fatalf("NewQmcCipherDecoder() error = %v", err)
	}
	cipher.Decrypt(encrypted, 0)

	path := filepath.Join(t.TempDir(), "sample.qmc0")
	if err := os.WriteFile(path, encrypted, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	reader, err := NewDecryptService(nil).DecryptFileByExt(path)
	if err != nil {
		t.Fatalf("DecryptFileByExt() error = %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("QMC decrypted bytes do not match the fixture")
	}
}

func TestDecryptFileByExtRejectsUnsupportedNewQmcVariants(t *testing.T) {
	_, err := NewDecryptService(nil).DecryptFileByExt("sample.mflac")
	if !errors.Is(err, ErrUnsupportedInput) {
		t.Fatalf("DecryptFileByExt() error = %v, want ErrUnsupportedInput", err)
	}
}

func buildKwmFixture(plain []byte) []byte {
	header := make([]byte, 0x400)
	copy(header, []byte("yeelion-kuwo-tme"))
	key := []byte{0x15, 0xcd, 0x5b, 0x07, 0, 0, 0, 0}
	copy(header[0x18:0x20], key)
	copy(header[0x30:0x38], []byte("320mp3\x00\x00"))

	keyString := strconv.FormatUint(binary.LittleEndian.Uint64(key), 10)
	const predefined = "MoOtOiTvINGwd2E6n0E1i7L5t2IoOoNk"
	mask := make([]byte, 32)
	for i := range mask {
		mask[i] = predefined[i] ^ keyString[i%len(keyString)]
	}

	encrypted := append([]byte(nil), plain...)
	for i := range encrypted {
		encrypted[i] ^= mask[i&0x1f]
	}
	return append(header, encrypted...)
}
