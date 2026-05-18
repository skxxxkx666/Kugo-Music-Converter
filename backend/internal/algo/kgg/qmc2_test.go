package kgg

import (
	"bytes"
	"math/rand"
	"testing"
)

// refMapMask 是 QMC2 map 掩码的独立参考实现（不依赖被测代码的任何内部
// 状态），直接编码 unlock-music 的算法规范，用于校验。
// 注意 (v<<r)|(v>>r) 为酷狗 QMC2 既定的非循环移位行为，刻意如此。
func refMapMask(key []byte, offset int) byte {
	if offset > 0x7FFF {
		offset %= 0x7FFF
	}
	idx := (offset*offset + 71214) % len(key)
	v := key[idx]
	r := ((byte(idx) & 0x7) + 4) % 8
	return (v << r) | (v >> r)
}

func refMapDecrypt(buf []byte, key []byte, offset uint64) {
	for i := range buf {
		buf[i] ^= refMapMask(key, int(offset)+i)
	}
}

func TestQMC2MapDecryptMatchesReference(t *testing.T) {
	// 覆盖多种密钥长度：旧实现用 128 项预计算 + 平铺，仅在 256 字节
	// 密钥时碰巧正确；非 256 长度可暴露平铺回归。
	for _, ksize := range []int{16, 128, 180, 256, 300, 512} {
		keySrc := make([]byte, ksize)
		for i := range keySrc {
			keySrc[i] = byte((i*29 + 17) & 0xFF)
		}
		q := newQMC2Map(keySrc)

		offsets := []uint64{0, 1, 127, 128, 0x7FFE, 0x7FFF, 0x8000, 0xFFFE, 0xFFFF, 0x12345}
		lengths := []int{1, 7, 8, 31, 128, 1024, 0x9000}
		for _, off := range offsets {
			for _, n := range lengths {
				src := make([]byte, n)
				for i := range src {
					src[i] = byte((i*13 + 9) & 0xFF)
				}
				got := append([]byte(nil), src...)
				want := append([]byte(nil), src...)

				q.Decrypt(got, off)
				refMapDecrypt(want, keySrc, off)
				if !bytes.Equal(got, want) {
					t.Fatalf("map decrypt mismatch ksize=%d off=%d len=%d", ksize, off, n)
				}
			}
		}
	}
}

// TestQMC2MapKnownVector 锁定一个手工推导的已知答案，独立于任何实现，
// 用于防止 rotate 公式被再次“修正”成真实循环移位。
func TestQMC2MapKnownVector(t *testing.T) {
	keySrc := make([]byte, 256)
	for i := range keySrc {
		keySrc[i] = byte((i*29 + 17) & 0xFF)
	}
	// offset=0: idx=(0+71214)%256=46; key[46]=(46*29+17)&0xFF=0x47
	// r=((46&7)+4)%8=2; mask=(0x47<<2)|(0x47>>2)=0x1C|0x11=0x1D
	q := newQMC2Map(keySrc)
	buf := []byte{0x00}
	q.Decrypt(buf, 0)
	if buf[0] != 0x1D {
		t.Fatalf("known vector mismatch: got 0x%02X want 0x1D", buf[0])
	}
}

// --- 独立 RC4 参考实现：转写自 unlock-music rc4Cipher，刻意不复用被测
// 代码的任何内部状态（box/hash/keystream），以便真正捕获实现回归。 ---

type refRC4 struct {
	box  []byte
	key  []byte
	hash uint32
	n    int
}

func newRefRC4(key []byte) *refRC4 {
	n := len(key)
	c := &refRC4{key: key, n: n}
	c.box = make([]byte, n)
	for i := 0; i < n; i++ {
		c.box[i] = byte(i)
	}
	j := 0
	for i := 0; i < n; i++ {
		j = (j + int(c.box[i]) + int(key[i%n])) % n
		c.box[i], c.box[j] = c.box[j], c.box[i]
	}
	c.hash = 1
	for i := 0; i < n; i++ {
		v := uint32(key[i])
		if v == 0 {
			continue
		}
		nh := c.hash * v
		if nh == 0 || nh <= c.hash {
			break
		}
		c.hash = nh
	}
	return c
}

func (c *refRC4) segSkip(id int) int {
	seed := int(c.key[id%c.n])
	idx := int64(float64(c.hash) / float64((id+1)*seed) * 100.0)
	return int(idx % int64(c.n))
}

func (c *refRC4) decrypt(src []byte, offset int) {
	const segSize = 5120
	const firstSize = 128
	toProcess := len(src)
	processed := 0
	mark := func(p int) bool {
		offset += p
		toProcess -= p
		processed += p
		return toProcess == 0
	}
	encA := func(buf []byte, off int) {
		box := make([]byte, c.n)
		copy(box, c.box)
		j, k := 0, 0
		skipLen := (off % segSize) + c.segSkip(off/segSize)
		for i := -skipLen; i < len(buf); i++ {
			j = (j + 1) % c.n
			k = (int(box[j]) + k) % c.n
			box[j], box[k] = box[k], box[j]
			if i >= 0 {
				buf[i] ^= box[(int(box[j])+int(box[k]))%c.n]
			}
		}
	}
	if offset < firstSize {
		bs := toProcess
		if bs > firstSize-offset {
			bs = firstSize - offset
		}
		for i := 0; i < bs; i++ {
			src[i] ^= c.key[c.segSkip(offset+i)]
		}
		if mark(bs) {
			return
		}
	}
	if offset%segSize != 0 {
		bs := toProcess
		if bs > segSize-offset%segSize {
			bs = segSize - offset%segSize
		}
		encA(src[processed:processed+bs], offset)
		if mark(bs) {
			return
		}
	}
	for toProcess > segSize {
		encA(src[processed:processed+segSize], offset)
		mark(segSize)
	}
	if toProcess > 0 {
		encA(src[processed:], offset)
	}
}

func TestQMC2RC4DecryptMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	// 关键：覆盖非 512 长度。旧实现（skip&0x1FF + 定长 keyStream）仅在
	// 密钥恰好 512 字节时碰巧正确，301/320/480/700 等长度必须能暴露回归。
	for _, ksize := range []int{256, 301, 320, 480, 512, 700} {
		key := make([]byte, ksize)
		if _, err := rng.Read(key); err != nil {
			t.Fatalf("rng key: %v", err)
		}
		for i := range key {
			if key[i] == 0 {
				key[i] = byte(i%251 + 1)
			}
		}
		q := newQMC2RC4(key)
		for _, off := range []uint64{0, 1, 0x7F, 0x80, 0x81, 0x1400, 0x1401, 0x2345, 0x14000} {
			for _, n := range []int{1, 5, 8, 55, 512, 5119, 5120, 5121, 12000} {
				src := make([]byte, n)
				if _, err := rng.Read(src); err != nil {
					t.Fatalf("rng src: %v", err)
				}
				got := append([]byte(nil), src...)
				want := append([]byte(nil), src...)
				q.Decrypt(got, off)
				newRefRC4(key).decrypt(want, int(off))
				if !bytes.Equal(got, want) {
					t.Fatalf("rc4 mismatch ksize=%d off=%d len=%d", ksize, off, n)
				}
			}
		}
	}
}

// TestQMC2RC4KnownVector 用固定密钥钉死一段 encASegment 路径（旧 bug 重灾区）
// 的 keystream。期望值由上面独立参考一次性捕获后写死；若实现回退为旧的
// skip&0x1FF / 定长 keyStream，此处必 FAIL。
func TestQMC2RC4KnownVector(t *testing.T) {
	key := make([]byte, 400) // 400 ≠ 512，旧实现在此长度必错
	for i := range key {
		key[i] = byte((i*7 + 3) & 0xFF)
	}
	want := []byte{
		0xc5, 0xda, 0x55, 0x61, 0xb1, 0xa5, 0x7e, 0xea,
		0x47, 0x05, 0x51, 0x8a, 0x81, 0x61, 0xde, 0x6a,
	}
	q := newQMC2RC4(key)
	buf := make([]byte, 16) // 全 0：异或结果即 keystream
	q.Decrypt(buf, 0x1400)  // offset 0x1400 → 进入 encASegment
	if !bytes.Equal(buf, want) {
		t.Fatalf("rc4 known-vector mismatch:\n got=%x\n want=%x", buf, want)
	}
}
