package kgg

import (
	"errors"
)

type QMC2Base interface {
	Decrypt(buf []byte, offset uint64)
}

// --- QMC2 MAP ---
type qmc2Map struct {
	key [128]byte
}

func newQMC2Map(key []byte) *qmc2Map {
	var q qmc2Map
	n := len(key)
	for i := 0; i < 128; i++ {
		j := (i*i + 71214) % n
		shift := (j + 4) % 8
		q.key[i] = byte((uint16(key[j])<<shift | uint16(key[j])>>uint(8-shift)) & 0xFF)
	}
	return &q
}

func (q *qmc2Map) Decrypt(buf []byte, offset uint64) {
	for i := range buf {
		var idx uint64
		if offset <= 0x7FFF {
			idx = offset
		} else {
			idx = offset % 0x7FFF
		}
		buf[i] ^= q.key[idx%uint64(len(q.key))]
		offset++
	}
}

// --- QMC2 RC4 ---
type qmc2RC4 struct {
	key       []byte
	hash      float64
	keyStream [0x1400 + 512]byte
}

func newQMC2RC4(key []byte) *qmc2RC4 {
	q := &qmc2RC4{key: append([]byte(nil), key...)}
	q.hash = rc4hash(key)
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
	process := int(minUint64(uint64(len(buf)), 0x80-offset))
	for i := 0; i < process; i++ {
		idx := int(getSegmentKey(q.hash, offset, q.key[offset%uint64(n)])) % n
		buf[i] ^= q.key[idx]
		offset++
	}
	return process
}

func (q *qmc2RC4) decryptOther(buf []byte, offset uint64) int {
	n := len(q.key)
	segIdx := offset / 0x1400
	segOff := offset % 0x1400
	skip := getSegmentKey(q.hash, segIdx, q.key[segIdx%uint64(n)]) & 0x1FF
	process := int(minUint64(uint64(len(buf)), 0x1400-segOff))
	stream := q.keyStream[skip+segOff:]
	for i := 0; i < process; i++ {
		buf[i] ^= stream[i]
	}
	return process
}

func getSegmentKey(hash float64, segmentID uint64, seed byte) uint64 {
	if seed == 0 {
		return 0
	}
	return uint64((hash / (float64(uint64(seed)) * float64(segmentID+1))) * 100.0)
}

func rc4hash(key []byte) float64 {
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

// --- EKey ---
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

func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
