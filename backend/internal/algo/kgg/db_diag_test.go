package kgg

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
	pcdb "unlock-music.dev/cli/algo/kgm/pc_kugou_db"
)

// TestDecryptCorrectnessViaTempFile 隔离验证：仅用本项目的 decryptPcDatabase
// 解密（不走会崩的 Deserialize），改用临时文件 file-based 打开读取，并与
// unlock-music pc_kugou_db.CachedDumpEKey 的结果逐条对照。
func TestDecryptCorrectnessViaTempFile(t *testing.T) {
	dbPath := os.Getenv("KGG_DB")
	if dbPath == "" {
		t.Skip("set KGG_DB")
	}

	buffer, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := decryptPcDatabase(buffer); err != nil {
		t.Fatalf("decryptPcDatabase: %v", err)
	}
	if string(buffer[:16]) != "SQLite format 3\x00" {
		t.Fatalf("decrypted header not SQLite: % x", buffer[:16])
	}
	t.Logf("decrypted OK, size=%d header=%q", len(buffer), string(buffer[:16]))

	tmp, err := os.CreateTemp("", "kggdiag_*.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(buffer); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	db, err := sql.Open("sqlite", tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT EncryptionKeyId, EncryptionKey FROM ShareFileItems
		WHERE EncryptionKeyId IS NOT NULL AND EncryptionKeyId != ''
		  AND EncryptionKey IS NOT NULL AND EncryptionKey != ''`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	mine := map[string]string{}
	for rows.Next() {
		var id, k string
		if err := rows.Scan(&id, &k); err == nil {
			mine[id] = k
		}
	}
	rows.Close()
	t.Logf("mine entries=%d", len(mine))

	oracle, err := pcdb.CachedDumpEKey(dbPath)
	if err != nil {
		t.Fatalf("oracle CachedDumpEKey: %v", err)
	}
	t.Logf("oracle entries=%d", len(oracle))

	if len(mine) != len(oracle) {
		t.Fatalf("entry count mismatch: mine=%d oracle=%d", len(mine), len(oracle))
	}
	for id, k := range oracle {
		if mine[id] != k {
			t.Fatalf("mismatch for %q", id)
		}
	}
	t.Logf("OK: decrypt byte-correct, %d ekeys match unlock-music oracle", len(mine))
}
