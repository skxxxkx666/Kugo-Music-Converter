package kgg

import (
	"crypto/aes"
	"crypto/cipher"
)

// stdlib backing for AES/CBC

func aesNewCipherImpl(key []byte) (aesBlock, error) { return aes.NewCipher(key) }

type cbcDec struct{ cipher.BlockMode }

func newCBCDecrypterImpl(b aesBlock, iv []byte) cbcDecrypter {
	bm := cipher.NewCBCDecrypter(b.(cipher.Block), iv)
	return cbcDec{bm}
}

func (c cbcDec) CryptBlocks(dst, src []byte) { c.BlockMode.CryptBlocks(dst, src) }
