package kgg

import (
	"encoding/base64"
)

var (
	ekeyV2Prefix = "UVFNdXNpYyBFbmNWMixLZXk6"
	ekeyV2Key1   = [16]byte{0x33, 0x38, 0x36, 0x5A, 0x4A, 0x59, 0x21, 0x40, 0x23, 0x2A, 0x24, 0x25, 0x5E, 0x26, 0x29, 0x28}
	ekeyV2Key2   = [16]byte{0x2A, 0x2A, 0x23, 0x21, 0x28, 0x23, 0x24, 0x25, 0x26, 0x5E, 0x61, 0x31, 0x63, 0x5A, 0x2C, 0x54}
)

func decryptEkey(ekey string) []byte {
	if len(ekey) >= len(ekeyV2Prefix) && ekey[:len(ekeyV2Prefix)] == ekeyV2Prefix {
		ekey = ekey[len(ekeyV2Prefix):]
		b := teaCBCDecrypt([]byte(ekey), ekeyV2Key1)
		b = teaCBCDecrypt(b, ekeyV2Key2)
		return decryptEkeyV1(string(b))
	}
	return decryptEkeyV1(ekey)
}

func decryptEkeyV1(ekey string) []byte {
	raw, err := base64.StdEncoding.DecodeString(ekey)
	if err != nil || len(raw) < 8 {
		return nil
	}
	teaKey := [4]uint32{
		0x69005600 | uint32(raw[0])<<16 | uint32(raw[1]),
		0x46003800 | uint32(raw[2])<<16 | uint32(raw[3]),
		0x2b002000 | uint32(raw[4])<<16 | uint32(raw[5]),
		0x15000b00 | uint32(raw[6])<<16 | uint32(raw[7]),
	}
	dec := teaCBCDecrypt(raw[8:], teaKey)
	if len(dec) == 0 {
		return nil
	}
	out := make([]byte, 8, 8+len(dec))
	copy(out, raw[:8])
	out = append(out, dec...)
	return out
}

// --- TEA CBC (We implement only decrypt path used by ekey) ---
func teaCBCDecrypt(cipher []byte, key interface{}) []byte {
	if len(cipher)%8 != 0 || len(cipher) < 16 {
		return nil
	}
	var k [4]uint32
	switch v := key.(type) {
	case [16]byte:
		// interpret as 4 uint32 in LE words from bytes
		k[0] = (uint32(v[0])<<24 | uint32(v[1])<<16 | uint32(v[2])<<8 | uint32(v[3]))
		k[1] = (uint32(v[4])<<24 | uint32(v[5])<<16 | uint32(v[6])<<8 | uint32(v[7]))
		k[2] = (uint32(v[8])<<24 | uint32(v[9])<<16 | uint32(v[10])<<8 | uint32(v[11]))
		k[3] = (uint32(v[12])<<24 | uint32(v[13])<<16 | uint32(v[14])<<8 | uint32(v[15]))
	case [4]uint32:
		k = v
	}

	var iv1, iv2 uint64
	header := make([]byte, 16)
	in := cipher
	decryptRound(header[0:8], in[0:8], &iv1, &iv2, &k)
	decryptRound(header[8:16], in[8:16], &iv1, &iv2, &k)
	in = in[16:]

	hdrSkip := 1 + int(header[0]&7) + 2
	realPlain := len(cipher) - hdrSkip - 7
	if realPlain < 0 {
		return nil
	}
	res := make([]byte, realPlain)
	copyLen := min(realPlain, 16-hdrSkip)
	copy(res, header[hdrSkip:hdrSkip+copyLen])
	p := copyLen
	for i := len(cipher) - 24; i > 0; i -= 8 { // decrypt remaining blocks
		if p+8 > realPlain {
			break
		}
		decryptRound(res[p:p+8], in[0:8], &iv1, &iv2, &k)
		in = in[8:]
		p += 8
	}
	if p < realPlain && len(in) >= 8 {
		decryptRound(header[8:16], in[0:8], &iv1, &iv2, &k)
		res[p] = header[8]
	}
	return res
}

func decryptRound(dst, block []byte, iv1, iv2 *uint64, key *[4]uint32) {
	iv1Next := beRead64(block)
	iv2Next := teaECBDecrypt(iv1Next^*iv2, key)
	plain := iv2Next ^ *iv1
	*iv1 = iv1Next
	*iv2 = iv2Next
	beWrite64(dst, plain)
}

func teaECBDecrypt(v uint64, key *[4]uint32) uint64 {
	y := uint32(v >> 32)
	z := uint32(v)
	sum := uint32(0)
	for i := 0; i < 16; i++ {
		sum += 0x9e3779b9
	}
	for i := 0; i < 16; i++ {
		z -= teaSingleRound(y, sum, key[2], key[3])
		y -= teaSingleRound(z, sum, key[0], key[1])
		sum -= 0x9e3779b9
	}
	return (uint64(y) << 32) | uint64(z)
}

func teaSingleRound(v, sum, k1, k2 uint32) uint32 {
	return ((v << 4) + k1) ^ (v + sum) ^ ((v >> 5) + k2)
}

func beRead64(b []byte) uint64 {
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

func beWrite64(dst []byte, v uint64) {
	dst[0] = byte(v >> 56)
	dst[1] = byte(v >> 48)
	dst[2] = byte(v >> 40)
	dst[3] = byte(v >> 32)
	dst[4] = byte(v >> 24)
	dst[5] = byte(v >> 16)
	dst[6] = byte(v >> 8)
	dst[7] = byte(v)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
