package qmcfile

import (
	"bytes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"testing"

	"golang.org/x/crypto/tea"

	"kugo-music-converter/internal/algo/kgg"
)

func TestOpenDecryptsBoundedChunkedStream(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
	}{
		{name: "MAP", keySize: 128},
		{name: "RC4", keySize: 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plain := syntheticAudio(3*5120 + 257)
			ekey := syntheticEKey(t, tt.keySize)
			decryptor, err := kgg.CreateQMC2(ekey)
			if err != nil {
				t.Fatalf("CreateQMC2() fixture error = %v", err)
			}
			encrypted := append([]byte(nil), plain...)
			decryptor.Decrypt(encrypted, 0)

			footer := makeMusicExFooter(t, "StreamMID", "stream.mflac")
			path := writeFixture(t, append(encrypted, footer...))
			stream, info, err := Open(path, ekey)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if info.AudioLen != int64(len(plain)) {
				t.Fatalf("AudioLen = %d, want %d", info.AudioLen, len(plain))
			}

			var got bytes.Buffer
			chunkSizes := []int{1, 127, 2, 4093, 513, 8191, 31}
			for i := 0; ; i++ {
				buf := make([]byte, chunkSizes[i%len(chunkSizes)])
				n, readErr := stream.Read(buf)
				got.Write(buf[:n])
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					t.Fatalf("Read() error = %v", readErr)
				}
			}
			if !bytes.Equal(got.Bytes(), plain) {
				t.Fatalf("decrypted stream mismatch: got %d bytes, want %d", got.Len(), len(plain))
			}
			if got.Len() != len(plain) || bytes.Contains(got.Bytes(), musicExMagic[:]) {
				t.Fatal("bounded stream emitted footer data")
			}
			if err := stream.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
			if err := stream.Close(); err != nil {
				t.Fatalf("second Close() error = %v", err)
			}
			if _, err := stream.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("Read() after Close error = %v, want os.ErrClosed", err)
			}
		})
	}
}

func TestOpenErrors(t *testing.T) {
	musicExPath := writeFixture(t, append(syntheticAudio(32), makeMusicExFooter(t, "MID", "song.mgg")...))

	stream, info, err := Open(musicExPath, "")
	if stream != nil || info.Kind != KindMusicEx || !errors.Is(err, ErrMissingEKey) {
		t.Fatalf("Open(empty ekey) = stream %v info %#v err %v", stream, info, err)
	}

	stream, info, err = Open(musicExPath, "not base64")
	if stream != nil || info.Kind != KindMusicEx || !errors.Is(err, ErrInvalidEKey) {
		t.Fatalf("Open(invalid ekey) = stream %v info %#v err %v", stream, info, err)
	}

	legacyPath := writeFixture(t, syntheticAudio(32))
	stream, info, err = Open(legacyPath, syntheticEKey(t, 128))
	if stream != nil || info.Kind != KindLegacy || !errors.Is(err, ErrUnsupportedFooter) {
		t.Fatalf("Open(legacy) = stream %v info %#v err %v", stream, info, err)
	}
}

func syntheticEKey(t *testing.T, keySize int) string {
	t.Helper()
	if keySize < 16 {
		t.Fatalf("key size %d is too small", keySize)
	}
	key := make([]byte, keySize)
	for i := range key {
		key[i] = byte(i%251 + 1)
	}

	simpleKey := [...]byte{0x69, 0x56, 0x46, 0x38, 0x2B, 0x20, 0x15, 0x0B}
	teaKey := make([]byte, 16)
	for i := range simpleKey {
		teaKey[i*2] = simpleKey[i]
		teaKey[i*2+1] = key[i]
	}
	block, err := tea.NewCipherWithRounds(teaKey, 32)
	if err != nil {
		t.Fatalf("tea.NewCipherWithRounds() error = %v", err)
	}

	raw := append([]byte(nil), key[:8]...)
	raw = append(raw, encryptTencentTEA(t, block, key[8:])...)
	return base64.StdEncoding.EncodeToString(raw)
}

func encryptTencentTEA(t *testing.T, block cipher.Block, payload []byte) []byte {
	t.Helper()
	const overhead = 1 + 2 + 7
	padLen := (block.BlockSize() - (len(payload)+overhead)%block.BlockSize()) % block.BlockSize()
	plain := make([]byte, len(payload)+overhead+padLen)
	plain[0] = 0xA8 | byte(padLen)
	for i := 0; i < padLen; i++ {
		plain[1+i] = byte(0x31 + i)
	}
	plain[1+padLen] = 0x51
	plain[2+padLen] = 0x52
	copy(plain[3+padLen:], payload)

	out := make([]byte, len(plain))
	block.Encrypt(out[:8], plain[:8])
	previousCipher := out[:8]
	previousState := plain[:8]
	for offset := 8; offset < len(plain); offset += 8 {
		state := make([]byte, 8)
		for i := range state {
			state[i] = plain[offset+i] ^ previousCipher[i]
		}
		block.Encrypt(out[offset:offset+8], state)
		for i := range previousState {
			out[offset+i] ^= previousState[i]
		}
		previousCipher = out[offset : offset+8]
		previousState = state
	}
	return out
}
