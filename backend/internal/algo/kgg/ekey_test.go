package kgg

import "testing"

func TestBytesToTEAKey(t *testing.T) {
	key := [16]byte{
		0x01, 0x02, 0x03, 0x04,
		0x11, 0x12, 0x13, 0x14,
		0x21, 0x22, 0x23, 0x24,
		0x31, 0x32, 0x33, 0x34,
	}

	got := bytesToTEAKey(key)
	want := [4]uint32{
		0x01020304,
		0x11121314,
		0x21222324,
		0x31323334,
	}

	if got != want {
		t.Fatalf("bytesToTEAKey() = %#v, want %#v", got, want)
	}
}

func TestTeaCBCDecryptRejectsInvalidCipher(t *testing.T) {
	key := [4]uint32{}

	if got := teaCBCDecrypt([]byte{1, 2, 3}, key); got != nil {
		t.Fatalf("expected nil for short cipher, got len=%d", len(got))
	}

	if got := teaCBCDecrypt(make([]byte, 8), key); got != nil {
		t.Fatalf("expected nil for short block cipher, got len=%d", len(got))
	}
}
