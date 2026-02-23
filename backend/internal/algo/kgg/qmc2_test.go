package kgg

import (
	"bytes"
	"math/rand"
	"testing"
)

func refMapDecrypt(buf []byte, key [128]byte, offset uint64) {
	for i := range buf {
		idx := offset
		if idx > 0x7FFF {
			idx %= 0x7FFF
		}
		buf[i] ^= key[idx%uint64(len(key))]
		offset++
	}
}

func TestQMC2MapDecryptMatchesReference(t *testing.T) {
	keySrc := make([]byte, 256)
	for i := range keySrc {
		keySrc[i] = byte((i*29 + 17) & 0xFF)
	}
	q := newQMC2Map(keySrc)

	offsets := []uint64{0, 1, 127, 128, 0x7FFE, 0x7FFF, 0x8000, 0x12345}
	lengths := []int{1, 7, 8, 31, 128, 1024}
	for _, off := range offsets {
		for _, n := range lengths {
			src := make([]byte, n)
			for i := range src {
				src[i] = byte((i*13 + 9) & 0xFF)
			}
			got := append([]byte(nil), src...)
			want := append([]byte(nil), src...)

			q.Decrypt(got, off)
			refMapDecrypt(want, q.key, off)
			if !bytes.Equal(got, want) {
				t.Fatalf("map decrypt mismatch at off=%d len=%d", off, n)
			}
		}
	}
}

func refRC4Hash(key []byte) float64 {
	var h uint32 = 1
	for _, b := range key {
		if b == 0 {
			continue
		}
		next := h * uint32(b)
		if next <= h {
			break
		}
		h = next
	}
	return float64(h)
}

func refGetSegmentKey(hash float64, segmentID uint64, seed byte) uint64 {
	if seed == 0 {
		return 0
	}
	return uint64((hash / (float64(uint64(seed)) * float64(segmentID+1))) * 100.0)
}

func refRC4Decrypt(buf []byte, offset uint64, key []byte, stream [0x1400 + 512]byte) {
	hash := refRC4Hash(key)
	if offset < 0x80 {
		n := int(min(uint64(len(buf)), 0x80-offset))
		for i := 0; i < n; i++ {
			idx := int(refGetSegmentKey(hash, offset, key[offset%uint64(len(key))])) % len(key)
			buf[i] ^= key[idx]
			offset++
		}
		buf = buf[n:]
	}
	for len(buf) > 0 {
		segIdx := offset / 0x1400
		segOff := offset % 0x1400
		skip := refGetSegmentKey(hash, segIdx, key[segIdx%uint64(len(key))]) & 0x1FF
		n := int(min(uint64(len(buf)), 0x1400-segOff))
		ref := stream[skip+segOff:]
		for i := 0; i < n; i++ {
			buf[i] ^= ref[i]
		}
		offset += uint64(n)
		buf = buf[n:]
	}
}

func TestQMC2RC4DecryptMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	key := make([]byte, 512)
	if _, err := rng.Read(key); err != nil {
		t.Fatalf("rng read key failed: %v", err)
	}
	for i := range key {
		if key[i] == 0 {
			key[i] = byte(i%251 + 1)
		}
	}
	q := newQMC2RC4(key)

	for _, off := range []uint64{0, 1, 0x7F, 0x80, 0x180, 0x2345} {
		for _, n := range []int{1, 5, 8, 55, 512, 4096} {
			src := make([]byte, n)
			if _, err := rng.Read(src); err != nil {
				t.Fatalf("rng read src failed: %v", err)
			}
			got := append([]byte(nil), src...)
			want := append([]byte(nil), src...)
			q.Decrypt(got, off)
			refRC4Decrypt(want, off, key, q.keyStream)
			if !bytes.Equal(got, want) {
				t.Fatalf("rc4 decrypt mismatch at off=%d len=%d", off, n)
			}
		}
	}
}
