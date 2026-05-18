package kgg

import (
	"errors"
)

type QMC2Base interface {
	// Decrypt decrypts buf in-place at the given stream offset.
	Decrypt(buf []byte, offset uint64)
}

// --- QMC2 MAP ---
//
// 实现与 unlock-music.dev/cli/algo/qmc 的 mapCipher 行为完全一致。
//
// 注意：getMask 中的位运算 (v<<r)|(v>>r) 并非标准循环移位 —— 左移与右移
// 使用同一个位移量 r 并在 8 位内截断。这是酷狗 / QQ 音乐 QMC2 的既定算法，
// 必须原样复刻。切勿“修正”为 (v<<r)|(v>>(8-r)) 的真实循环移位，否则除首
// 字节外的解密结果全部错误（历史 bug：曾导致所有 MAP 模式 KGG 解密失败）。
type qmc2Map struct {
	key  []byte
	size int
}

func newQMC2Map(key []byte) *qmc2Map {
	k := append([]byte(nil), key...)
	return &qmc2Map{key: k, size: len(k)}
}

func (q *qmc2Map) getMask(offset int) byte {
	if offset > 0x7FFF {
		offset %= 0x7FFF
	}
	idx := (offset*offset + 71214) % q.size
	v := q.key[idx]
	r := ((byte(idx) & 0x7) + 4) % 8
	return (v << r) | (v >> r)
}

func (q *qmc2Map) Decrypt(buf []byte, offset uint64) {
	base := int(offset)
	for i := range buf {
		buf[i] ^= q.getMask(base + i)
	}
}

// --- QMC2 RC4 ---
//
// 忠实移植自 unlock-music.dev/cli/algo/qmc 的 rc4Cipher。
//
// 关键点（历史 bug：旧实现用定长预计算 keyStream + skip&0x1FF，仅在密钥恰好
// 512 字节时碰巧正确，其它 RC4 密钥长度全部解密错误）：
//   - getSegmentSkip 使用 idx % len(key)（而非 & 0x1FF）；
//   - 每个段（encASegment）都以 KSA box 的副本重新跑 PRGA，先丢弃
//     skipLen=(offset%5120)+getSegmentSkip(id) 个输出再异或；
//   - getSegmentSkip 的零 seed 行为刻意与 unlock-music 保持一致（不额外保护），
//     以保证与标准实现逐字节等价。
const (
	rc4SegmentSize      = 5120
	rc4FirstSegmentSize = 128
)

type qmc2RC4 struct {
	box  []byte
	key  []byte
	hash uint32
	n    int
}

func newQMC2RC4(key []byte) *qmc2RC4 {
	n := len(key)
	c := &qmc2RC4{key: append([]byte(nil), key...), n: n}
	c.box = make([]byte, n)
	for i := 0; i < n; i++ {
		c.box[i] = byte(i)
	}
	j := 0
	for i := 0; i < n; i++ {
		j = (j + int(c.box[i]) + int(c.key[i%n])) % n
		c.box[i], c.box[j] = c.box[j], c.box[i]
	}
	c.getHashBase()
	return c
}

func (c *qmc2RC4) getHashBase() {
	c.hash = 1
	for i := 0; i < c.n; i++ {
		v := uint32(c.key[i])
		if v == 0 {
			continue
		}
		nextHash := c.hash * v
		if nextHash == 0 || nextHash <= c.hash {
			break
		}
		c.hash = nextHash
	}
}

func (c *qmc2RC4) Decrypt(buf []byte, offset uint64) {
	src := buf
	off := int(offset)
	toProcess := len(src)
	processed := 0
	markProcess := func(p int) (finished bool) {
		off += p
		toProcess -= p
		processed += p
		return toProcess == 0
	}

	if off < rc4FirstSegmentSize {
		blockSize := toProcess
		if blockSize > rc4FirstSegmentSize-off {
			blockSize = rc4FirstSegmentSize - off
		}
		c.encFirstSegment(src[:blockSize], off)
		if markProcess(blockSize) {
			return
		}
	}

	if off%rc4SegmentSize != 0 {
		blockSize := toProcess
		if blockSize > rc4SegmentSize-off%rc4SegmentSize {
			blockSize = rc4SegmentSize - off%rc4SegmentSize
		}
		c.encASegment(src[processed:processed+blockSize], off)
		if markProcess(blockSize) {
			return
		}
	}
	for toProcess > rc4SegmentSize {
		c.encASegment(src[processed:processed+rc4SegmentSize], off)
		markProcess(rc4SegmentSize)
	}

	if toProcess > 0 {
		c.encASegment(src[processed:], off)
	}
}

func (c *qmc2RC4) encFirstSegment(buf []byte, offset int) {
	for i := 0; i < len(buf); i++ {
		buf[i] ^= c.key[c.getSegmentSkip(offset+i)]
	}
}

func (c *qmc2RC4) encASegment(buf []byte, offset int) {
	box := make([]byte, c.n)
	copy(box, c.box)
	j, k := 0, 0

	skipLen := (offset % rc4SegmentSize) + c.getSegmentSkip(offset/rc4SegmentSize)
	for i := -skipLen; i < len(buf); i++ {
		j = (j + 1) % c.n
		k = (int(box[j]) + k) % c.n
		box[j], box[k] = box[k], box[j]
		if i >= 0 {
			buf[i] ^= box[(int(box[j])+int(box[k]))%c.n]
		}
	}
}

func (c *qmc2RC4) getSegmentSkip(id int) int {
	seed := int(c.key[id%c.n])
	idx := int64(float64(c.hash) / float64((id+1)*seed) * 100.0)
	return int(idx % int64(c.n))
}

// --- EKey ---
// CreateQMC2 builds the QMC2 decryptor from decrypted ekey payload.
// 与 unlock-music 一致：len(key) > 300 用 RC4，否则用 MAP。
func CreateQMC2(ekey string) (QMC2Base, error) {
	key := decryptEkey(ekey)
	if len(key) == 0 {
		return nil, errors.New("invalid ekey")
	}
	if len(key) > 300 {
		return newQMC2RC4(key), nil
	}
	return newQMC2Map(key), nil
}
