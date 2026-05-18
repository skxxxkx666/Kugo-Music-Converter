package kgg

import (
	"encoding/base64"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ucommon "unlock-music.dev/cli/algo/common"
	ukgm "unlock-music.dev/cli/algo/kgm"
)

// TestAcceptanceBatch 对 KGG_DIR 下所有 .kgg 做最终验收：
// 分类（V1/V2、MAP/RC4），并把本项目完整解密结果与 unlock-music kgm v5
// 标准解码器逐字节对照。env 门控：设 KGG_DIR（默认 KGG_DB 自动检测）。
func TestAcceptanceBatch(t *testing.T) {
	dir := os.Getenv("KGG_DIR")
	dbPath := os.Getenv("KGG_DB")
	if dir == "" || dbPath == "" {
		t.Skip("set KGG_DIR and KGG_DB to run batch acceptance")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	keyMap, err := LoadKGDatabaseKeyMap(dbPath)
	if err != nil {
		t.Fatalf("LoadKGDatabaseKeyMap: %v", err)
	}
	t.Logf("DB key map entries: %d", len(keyMap))

	var total, ok, mapMode, rc4Mode, v1, v2 int
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".kgg") {
			continue
		}
		total++
		name := e.Name()
		path := filepath.Join(dir, name)

		full, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Errorf("[%s] read: %v", name, rerr)
			continue
		}
		headerLen := int64(binary.LittleEndian.Uint32(full[16:20]))
		hashLen := int(binary.LittleEndian.Uint32(full[68:72]))
		audioHash := string(full[72 : 72+hashLen])
		ekey, found := keyMap[audioHash]
		if !found {
			t.Errorf("[%s] ekey NOT found (audioHash=%s) — 请在酷狗客户端播放一次后重载 DB", name, audioHash)
			continue
		}

		// 分类：V1/V2
		ver := "V1"
		if dec, derr := base64.StdEncoding.DecodeString(ekey); derr == nil &&
			strings.HasPrefix(string(dec), "QQMusic EncV2,Key:") {
			ver = "V2"
			v2++
		} else {
			v1++
		}
		keyBytes := decryptEkey(ekey)
		mode := "MAP"
		if len(keyBytes) > 300 {
			mode = "RC4"
			rc4Mode++
		} else {
			mapMode++
		}

		// 项目完整解密
		cipher, cerr := CreateQMC2(ekey)
		if cerr != nil {
			t.Errorf("[%s] CreateQMC2: %v", name, cerr)
			continue
		}
		mine := append([]byte(nil), full[headerLen:]...)
		cipher.Decrypt(mine, 0)

		// unlock-music 标准解码器
		f, oerr := os.Open(path)
		if oerr != nil {
			t.Errorf("[%s] open: %v", name, oerr)
			continue
		}
		dec := ukgm.NewDecoder(&ucommon.DecoderParams{
			Reader: f, Extension: ".kgg", FilePath: path, KggDatabasePath: dbPath,
		})
		if verr := dec.Validate(); verr != nil {
			f.Close()
			t.Errorf("[%s] oracle validate: %v", name, verr)
			continue
		}
		oracle, aerr := io.ReadAll(dec)
		f.Close()
		if aerr != nil {
			t.Errorf("[%s] oracle read: %v", name, aerr)
			continue
		}

		status := "OK byte-identical"
		match := len(mine) == len(oracle)
		if match {
			for i := range mine {
				if mine[i] != oracle[i] {
					match = false
					t.Errorf("[%s] MISMATCH at offset %d (mine=0x%02X oracle=0x%02X)", name, i, mine[i], oracle[i])
					status = "MISMATCH"
					break
				}
			}
		} else {
			t.Errorf("[%s] length mismatch mine=%d oracle=%d", name, len(mine), len(oracle))
			status = "LEN-MISMATCH"
		}
		if match {
			ok++
		}
		t.Logf("[%-2s/%-3s] keyRaw=%-4d keyDec=%-4d %-22s | %s",
			ver, mode, len(ekey), len(keyBytes), truncName(name), status)
	}

	t.Logf("==== 验收汇总：文件 %d，逐字节通过 %d；模式 MAP=%d RC4=%d；ekey V1=%d V2=%d ====",
		total, ok, mapMode, rc4Mode, v1, v2)
	if ok != total {
		t.Fatalf("acceptance FAILED: %d/%d byte-identical", ok, total)
	}
}

func truncName(s string) string {
	r := []rune(s)
	if len(r) <= 22 {
		return s
	}
	return string(r[:21]) + "…"
}
