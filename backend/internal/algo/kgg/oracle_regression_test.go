package kgg

import (
	"encoding/binary"
	"io"
	"os"
	"testing"

	ucommon "unlock-music.dev/cli/algo/common"
	ukgm "unlock-music.dev/cli/algo/kgm"
)

// TestOracleRealFileRegression 用真实 .kgg + 真实 KGMusicV3.db，把本项目的
// 完整解密结果与 unlock-music 的 kgm v5 解码器（标准答案）逐字节对照。
//
// 默认跳过；设置环境变量后启用：
//
//	$env:KGG_DB="...\KGMusicV3.db"; $env:KGG_FILE="...\song.kgg"
//	go test ./internal/algo/kgg/ -run TestOracleRealFileRegression -v
//
// 建议样本至少覆盖：V1+MAP（已验证）、RC4 模式（密钥≠512）、EncV2 前缀。
func TestOracleRealFileRegression(t *testing.T) {
	dbPath := os.Getenv("KGG_DB")
	filePath := os.Getenv("KGG_FILE")
	if dbPath == "" || filePath == "" {
		t.Skip("set KGG_DB and KGG_FILE to run the real-file oracle regression")
	}

	// 1. 标准答案：unlock-music kgm v5 解码器
	of, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer of.Close()
	dec := ukgm.NewDecoder(&ucommon.DecoderParams{
		Reader:          of,
		Extension:       ".kgg",
		FilePath:        filePath,
		KggDatabasePath: dbPath,
	})
	if err := dec.Validate(); err != nil {
		t.Fatalf("oracle validate: %v", err)
	}
	oracle, err := io.ReadAll(dec)
	if err != nil {
		t.Fatalf("oracle read: %v", err)
	}

	// 2. 本项目：解 DB 取 ekey -> CreateQMC2 -> 整文件解密
	keyMap, err := LoadKGDatabaseKeyMap(dbPath)
	if err != nil {
		t.Fatalf("LoadKGDatabaseKeyMap: %v", err)
	}

	full, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	headerLen := int64(binary.LittleEndian.Uint32(full[16:20]))
	hashLen := int(binary.LittleEndian.Uint32(full[68:72]))
	audioHash := string(full[72 : 72+hashLen])
	ekey, ok := keyMap[audioHash]
	if !ok {
		t.Fatalf("audioHash %q not found in DB (key expired? play the song in Kugou and reload)", audioHash)
	}
	cipher, err := CreateQMC2(ekey)
	if err != nil {
		t.Fatalf("CreateQMC2: %v", err)
	}
	body := append([]byte(nil), full[headerLen:]...)
	cipher.Decrypt(body, 0)

	if len(body) != len(oracle) {
		t.Fatalf("length mismatch: project=%d oracle=%d", len(body), len(oracle))
	}
	for i := range body {
		if body[i] != oracle[i] {
			t.Fatalf("byte mismatch at offset %d: project=0x%02X oracle=0x%02X", i, body[i], oracle[i])
		}
	}
	t.Logf("OK: %d bytes byte-identical to unlock-music oracle (ekey raw len=%d)", len(body), len(ekey))
}
