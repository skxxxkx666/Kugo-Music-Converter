package kgg

import (
	"encoding/binary"
	"errors"
)

type QMC2Base interface {
	// Decrypt decrypts buf in-place at the given stream offset.
	Decrypt(buf []byte, offset uint64)
}

// --- QMC2 MAP ---
type qmc2Map struct {
	key   [128]byte
	first [0x8000]byte
	loop  [0x7fff]byte
}

func newQMC2Map(key []byte) *qmc2Map {
	var q qmc2Map
	n := len(key)
	for i := 0; i < 128; i++ {
		j := (i*i + 71214) % n
		shift := (j + 4) % 8
		q.key[i] = byte((uint16(key[j])<<shift | uint16(key[j])>>uint(8-shift)) & 0xFF)
	}
	for i := range q.first {
		q.first[i] = q.key[i%len(q.key)]
	}
	for i := range q.loop {
		q.loop[i] = q.key[i%len(q.key)]
	}
	return &q
}

func (q *qmc2Map) Decrypt(buf []byte, offset uint64) {
	if len(buf) == 0 {
		return
	}

	if offset <= 0x7FFF {
		firstRemain := int(0x8000 - offset)
		n := min(len(buf), firstRemain)
		xorWithTable(buf[:n], q.first[:], int(offset))
		buf = buf[n:]
		offset += uint64(n)
	}

	if len(buf) == 0 {
		return
	}
	xorWithTable(buf, q.loop[:], int(offset%0x7fff))
}

// --- QMC2 RC4 ---
type qmc2RC4 struct {
	key       []byte
	hash100   uint64
	keyStream [0x1400 + 512]byte
}

func newQMC2RC4(key []byte) *qmc2RC4 {
	q := &qmc2RC4{key: append([]byte(nil), key...)}
	q.hash100 = rc4hash100(key)
	// derive stream
	var rc rc4KeySched
	rc.init(key)
	rc.derive(q.keyStream[:])
	return q
}

func (q *qmc2RC4) Decrypt(buf []byte, offset uint64) {
	if offset < 0x80 { // first segment
		n := q.decryptFirst(buf, offset)
		offset += uint64(n)
		buf = buf[n:]
	}
	for len(buf) > 0 {
		n := q.decryptOther(buf, offset)
		offset += uint64(n)
		buf = buf[n:]
	}
}

func (q *qmc2RC4) decryptFirst(buf []byte, offset uint64) int {
	n := len(q.key)
	process := int(min(uint64(len(buf)), 0x80-offset))
	for i := 0; i < process; i++ {
		idx := int(getSegmentKey(q.hash100, offset, q.key[offset%uint64(n)]) % uint64(n))
		buf[i] ^= q.key[idx]
		offset++
	}
	return process
}

func (q *qmc2RC4) decryptOther(buf []byte, offset uint64) int {
	n := len(q.key)
	segIdx := offset / 0x1400
	segOff := offset % 0x1400
	skip := getSegmentKey(q.hash100, segIdx, q.key[segIdx%uint64(n)]) & 0x1FF
	process := int(min(uint64(len(buf)), 0x1400-segOff))
	streamStart := int(skip + segOff)
	stream := q.keyStream[streamStart:]
	xorContiguous(buf[:process], stream[:process])
	return process
}

func getSegmentKey(hash100 uint64, segmentID uint64, seed byte) uint64 {
	if seed == 0 {
		return 0
	}
	return hash100 / (uint64(seed) * (segmentID + 1))
}

func rc4hash100(key []byte) uint64 {
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
	return uint64(h) * 100
}

// --- RC4 KSA/PRGA (only derive keystream) ---
type rc4KeySched struct {
	s    []byte
	i, j int
}

func (r *rc4KeySched) init(key []byte) {
	n := len(key)
	r.s = make([]byte, n)
	for i := 0; i < n; i++ {
		r.s[i] = byte(i)
	}
	j := 0
	for i := 0; i < n; i++ {
		j = (j + int(r.s[i]) + int(key[i])) % n
		r.s[i], r.s[j] = r.s[j], r.s[i]
	}
	r.i, r.j = 0, 0
}

func (r *rc4KeySched) derive(out []byte) {
	n := len(r.s)
	i, j := r.i, r.j
	s := r.s
	for k := range out {
		i = (i + 1) % n
		j = (j + int(s[i])) % n
		s[i], s[j] = s[j], s[i]
		out[k] ^= s[(int(s[i])+int(s[j]))%n]
	}
	r.i, r.j = i, j
}

func xorContiguous(dst, key []byte) {
	i := 0
	for ; i+8 <= len(dst); i += 8 {
		v := binary.LittleEndian.Uint64(dst[i:]) ^ binary.LittleEndian.Uint64(key[i:])
		binary.LittleEndian.PutUint64(dst[i:], v)
	}
	for ; i < len(dst); i++ {
		dst[i] ^= key[i]
	}
}

func xorWithTable(dst, table []byte, start int) {
	if len(dst) == 0 || len(table) == 0 {
		return
	}
	idx := start % len(table)
	for len(dst) > 0 {
		n := min(len(dst), len(table)-idx)
		xorContiguous(dst[:n], table[idx:idx+n])
		dst = dst[n:]
		idx = 0
	}
}

// --- EKey ---
// CreateQMC2 builds the QMC2 decryptor from decrypted ekey payload.
// Short keys (<300 bytes) use map mode; longer keys use rc4 mode.
func CreateQMC2(ekey string) (QMC2Base, error) {
	key := decryptEkey(ekey)
	if len(key) == 0 {
		return nil, errors.New("invalid ekey")
	}
	if len(key) < 300 {
		return newQMC2Map(key), nil
	}
	return newQMC2RC4(key), nil
}
