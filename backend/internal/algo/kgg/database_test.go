package kgg

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

func TestAesCBCDecrypt(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("fedcba9876543210")
	plain := []byte("1234567890ABCDEF")
	enc := make([]byte, len(plain))

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher() error = %v", err)
	}
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(enc, plain)

	got, err := aesCBCDecrypt(enc, key, iv)
	if err != nil {
		t.Fatalf("aesCBCDecrypt() unexpected error = %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("aesCBCDecrypt() = %x, want %x", got, plain)
	}
}

func TestAesCBCDecryptInvalidKey(t *testing.T) {
	_, err := aesCBCDecrypt(make([]byte, 16), []byte("bad"), make([]byte, 16))
	if err == nil {
		t.Fatal("expected error for invalid key length")
	}
}

func TestAesCBCDecryptUnalignedCipher(t *testing.T) {
	_, err := aesCBCDecrypt(make([]byte, 15), []byte("0123456789abcdef"), make([]byte, 16))
	if err == nil {
		t.Fatal("expected error for unaligned cipher length")
	}
	if err.Error() != "cipher length is not aligned with block size" {
		t.Fatalf("unexpected error: %v", err)
	}
}
