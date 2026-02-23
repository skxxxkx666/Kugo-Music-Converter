package service

import (
	"errors"
	"os"
	"strings"
	"testing"
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
