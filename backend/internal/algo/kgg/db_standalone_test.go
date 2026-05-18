package kgg

import (
	"encoding/binary"
	"os"
	"testing"
)

// TestLoadKGDatabaseKeyMapStandalone 仅验证本项目自有的内存解密链路
// （不引入 unlock-music，避免同进程两次 Deserialize-FREEONCLOSE 冲突）。
// env 门控：设 KGG_DB / KGG_FILE 启用。
func TestLoadKGDatabaseKeyMapStandalone(t *testing.T) {
	dbPath := os.Getenv("KGG_DB")
	filePath := os.Getenv("KGG_FILE")
	if dbPath == "" || filePath == "" {
		t.Skip("set KGG_DB and KGG_FILE")
	}

	m, err := LoadKGDatabaseKeyMap(dbPath)
	if err != nil {
		t.Fatalf("LoadKGDatabaseKeyMap: %v", err)
	}
	if len(m) == 0 {
		t.Fatalf("empty key map")
	}
	t.Logf("key map entries: %d", len(m))

	full, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	headerLen := int64(binary.LittleEndian.Uint32(full[16:20]))
	hashLen := int(binary.LittleEndian.Uint32(full[68:72]))
	audioHash := string(full[72 : 72+hashLen])
	ekey, ok := m[audioHash]
	if !ok {
		t.Fatalf("audioHash %q not found", audioHash)
	}
	cipher, err := CreateQMC2(ekey)
	if err != nil {
		t.Fatalf("CreateQMC2: %v", err)
	}
	body := append([]byte(nil), full[headerLen:headerLen+64]...)
	cipher.Decrypt(body, 0)
	// FLAC 头应为 "fLaC"
	if !(body[0] == 'f' && body[1] == 'L' && body[2] == 'a' && body[3] == 'C') {
		t.Fatalf("decrypt head not fLaC: % x", body[:8])
	}
	t.Logf("OK standalone decrypt head: %s", string(body[:4]))
}

// TestLoadKGDatabaseKeyMapRepeated 验证同进程内多次 Deserialize+close 稳定
// （生产中用户播放新歌后重载 DB 会触发再次加载，不能崩溃）。
func TestLoadKGDatabaseKeyMapRepeated(t *testing.T) {
	dbPath := os.Getenv("KGG_DB")
	if dbPath == "" {
		t.Skip("set KGG_DB")
	}
	var n int
	for i := 0; i < 5; i++ {
		m, err := LoadKGDatabaseKeyMap(dbPath)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		if i == 0 {
			n = len(m)
		} else if len(m) != n {
			t.Fatalf("iter %d: entries changed %d -> %d", i, n, len(m))
		}
	}
	t.Logf("OK repeated x5, entries=%d", n)
}
