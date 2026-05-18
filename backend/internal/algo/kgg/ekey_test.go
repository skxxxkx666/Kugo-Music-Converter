package kgg

import (
	"bytes"
	"testing"
)

// TestSimpleMakeKeyKnownConstants 钉死 simpleMakeKey(106,8)。
// 这 8 个值即旧版（经真实 V1 文件 oracle 验证正确）硬编码进 TEA key 偶数位
// 的常量，作为独立交叉校验，防止 math.Tan 派生被误改。
func TestSimpleMakeKeyKnownConstants(t *testing.T) {
	got := simpleMakeKey(106, 8)
	want := []byte{0x69, 0x56, 0x46, 0x38, 0x2b, 0x20, 0x15, 0x0b}
	if !bytes.Equal(got, want) {
		t.Fatalf("simpleMakeKey(106,8) = %x, want %x", got, want)
	}
}

func TestDecryptTencentTeaRejectsInvalid(t *testing.T) {
	key := make([]byte, 16)

	if _, err := decryptTencentTea([]byte{1, 2, 3}, key); err == nil {
		t.Fatal("expected error for non-block-aligned input")
	}
	if _, err := decryptTencentTea(make([]byte, 8), key); err == nil {
		t.Fatal("expected error for too-small input")
	}
}

func TestDeriveKeyRejectsBadBase64(t *testing.T) {
	if k := decryptEkey("not valid base64 @@@"); k != nil {
		t.Fatalf("expected nil for invalid base64 ekey, got len=%d", len(k))
	}
	if k := decryptEkey(""); k != nil {
		t.Fatalf("expected nil for empty ekey, got len=%d", len(k))
	}
}

// 注：V1 端到端正确性由 TestOracleRealFileRegression（真实 .kgg + DB，与
// unlock-music kgm v5 逐字节对照）覆盖。V2(EncV2) 需真实样本验证，见
// docs/PLAN-V0.5.0.md 第八节"前置依赖"。
