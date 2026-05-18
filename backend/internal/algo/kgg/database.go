package kgg

// KGMusicV3.db 解密：忠实移植自 unlock-music.dev/cli/algo/kgm/pc_kugou_db。
//
// 相比旧实现的改进（F-5004）：
//   - 全程在内存中解密 + 通过 modernc SQLite Deserialize 读取，
//     不再把明文密钥库写入 %TEMP%（消除 DRM 密钥落盘/残留风险）；
//   - 页 1 解密后做完整性校验，失败给出明确错误（便于定位 master key 失配
//     或不支持的库版本，而非 "打不开数据库" 的模糊报错）；
//   - master key 支持多候选 + 环境变量覆盖（KGG_DB_MASTER_KEY，hex），
//     为酷狗后续轮换 master key 提供降级空间。

import (
	"bytes"
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	_ "modernc.org/sqlite"
)

const dbPageSize = 0x400

var sqliteHeader = []byte("SQLite format 3\x00")

// defaultMasterKey 为已知 master key（16 字节）。
var defaultMasterKey = [16]byte{
	0x1D, 0x61, 0x31, 0x45, 0xB2, 0x47, 0xBF, 0x7F,
	0x3D, 0x18, 0x96, 0x72, 0x14, 0x4F, 0xE4, 0xBF,
}

// masterKeyCandidates 返回待尝试的 master key 列表：环境变量覆盖优先，
// 其后为内置已知 key。任一候选能让页 1 通过完整性校验即采用。
func masterKeyCandidates() [][16]byte {
	out := make([][16]byte, 0, 2)
	if env := strings.TrimSpace(os.Getenv("KGG_DB_MASTER_KEY")); env != "" {
		if raw, err := hex.DecodeString(env); err == nil && len(raw) == 16 {
			var k [16]byte
			copy(k[:], raw)
			out = append(out, k)
		}
	}
	out = append(out, defaultMasterKey)
	return out
}

func nextPageIV(seed uint32) uint32 {
	left := seed * 0x9EF4
	right := seed / 0xCE26 * 0x7FFFFF07
	value := left - right
	if value&0x80000000 == 0 {
		return value
	}
	return value + 0x7FFFFF07
}

func derivePageAESKey(seed uint32, master [16]byte) []byte {
	buf := make([]byte, 0x18)
	copy(buf[:0x10], master[:])
	binary.LittleEndian.PutUint32(buf[0x10:0x14], seed)
	// 固定值 0x73416C54（"sAlT" LE）
	binary.LittleEndian.PutUint32(buf[0x14:0x18], 0x546C4173)
	digest := md5.Sum(buf)
	return digest[:]
}

func derivePageAESIV(seed uint32) []byte {
	iv := make([]byte, 0x10)
	seed = seed + 1
	for i := 0; i < 0x10; i += 4 {
		seed = nextPageIV(seed)
		binary.LittleEndian.PutUint32(iv[i:i+4], seed)
	}
	digest := md5.Sum(iv)
	return digest[:]
}

func decryptDBPage(buffer []byte, pageNumber uint32, master [16]byte) error {
	key := derivePageAESKey(pageNumber, master)
	iv := derivePageAESIV(pageNumber)
	return aesCBCDecryptTo(buffer, buffer, key, iv)
}

func validatePage1Header(header []byte) error {
	if len(header) < 0x18 {
		return errors.New("kgg db: page 1 too short")
	}
	o10 := binary.LittleEndian.Uint32(header[0x10:0x14])
	o14 := binary.LittleEndian.Uint32(header[0x14:0x18])
	v6 := ((o10 & 0xff) << 8) | ((o10 & 0xff00) << 16)
	ok := o14 == 0x20204000 && (v6-0x200) <= 0xFE00 && ((v6-1)&v6) == 0
	if !ok {
		return errors.New("kgg db: invalid page 1 header")
	}
	return nil
}

// decryptPage1 解密首页并做完整性校验；返回 nil 表示该 master 正确。
func decryptPage1(page []byte, master [16]byte) error {
	if err := validatePage1Header(page); err != nil {
		return err
	}

	expectedHdr := make([]byte, 8)
	copy(expectedHdr, page[0x10:0x18])

	hdr := page[:0x10]
	copy(page[0x10:0x18], hdr[0x08:0x10])

	if err := decryptDBPage(page[0x10:], 1, master); err != nil {
		return err
	}

	if !bytes.Equal(page[0x10:0x18], expectedHdr) {
		return errors.New("kgg db: page 1 integrity check failed")
	}

	copy(hdr, sqliteHeader)
	return nil
}

// decryptPcDatabase 就地解密整个数据库；自动跳过未加密库；逐个尝试候选
// master key（仅页 1 校验通过的 master 才用于后续页）。
func decryptPcDatabase(buffer []byte) error {
	if len(buffer) >= len(sqliteHeader) && bytes.Equal(buffer[:len(sqliteHeader)], sqliteHeader) {
		return nil // 未加密
	}
	if len(buffer) == 0 || len(buffer)%dbPageSize != 0 {
		return fmt.Errorf("kgg db: invalid database size: %d", len(buffer))
	}

	lastPage := len(buffer) / dbPageSize

	var chosen *[16]byte
	var lastErr error
	for _, m := range masterKeyCandidates() {
		trial := append([]byte(nil), buffer[:dbPageSize]...)
		mk := m
		if err := decryptPage1(trial, mk); err != nil {
			lastErr = err
			continue
		}
		copy(buffer[:dbPageSize], trial)
		chosen = &mk
		break
	}
	if chosen == nil {
		return fmt.Errorf("kgg db: decrypt page 1 failed for all master key candidates: %w", lastErr)
	}

	offset := dbPageSize
	for pageNo := 2; pageNo <= lastPage; pageNo++ {
		if err := decryptDBPage(buffer[offset:offset+dbPageSize], uint32(pageNo), *chosen); err != nil {
			return fmt.Errorf("kgg db: decrypt page %d failed: %w", pageNo, err)
		}
		offset += dbPageSize
	}
	return nil
}

// extractKeyMapping 把解密后的 SQLite 字节通过内存 Deserialize 加载并读取
// EncryptionKeyId -> EncryptionKey 映射（不落盘）。
var memDBSeq atomic.Uint64

// extractKeyMapping 把解密后的 SQLite 字节通过内存 Deserialize 加载并读取
// 映射，全程不落盘。沿用 unlock-music 已验证的“共享缓存 + 先在一个连接上
// Deserialize 后关闭、再用新连接查询”模式；每次调用使用唯一的内存库名，
// 使其相互隔离并能干净销毁（避免 modernc v1.37.0 在 db.Close() 时于
// _sqlite3SchemaClear 触发的非法内存访问）。
func extractKeyMapping(buffer []byte) (map[string]string, error) {
	dsn := fmt.Sprintf("file:kggdb_%d?mode=memory&cache=shared", memDBSeq.Add(1))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	conn, err := db.Conn(context.Background())
	if err != nil {
		return nil, err
	}
	if err := func() error {
		defer conn.Close()
		return conn.Raw(func(driverConn any) error {
			type serializer interface {
				Serialize() ([]byte, error)
				Deserialize([]byte) error
			}
			s, ok := driverConn.(serializer)
			if !ok {
				return errors.New("kgg db: sqlite driver does not support Deserialize")
			}
			return s.Deserialize(buffer)
		})
	}(); err != nil {
		return nil, fmt.Errorf("kgg db: import decrypted db failed: %w", err)
	}

	// 注意：第二个连接刻意不显式 Close —— 与 unlock-music 一致，由 db.Close()
	// 统一销毁共享缓存内存库。若在此提前 conn.Close()，modernc v1.37.0 会在
	// _sqlite3SchemaClear 触发非法内存访问。
	conn, err = db.Conn(context.Background())
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(context.Background(),
		`SELECT EncryptionKeyId, EncryptionKey FROM ShareFileItems
		 WHERE EncryptionKeyId IS NOT NULL AND EncryptionKeyId != ''
		   AND EncryptionKey   IS NOT NULL AND EncryptionKey   != ''`)
	if err != nil {
		return nil, fmt.Errorf("kgg db: query ShareFileItems failed: %w", err)
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			continue
		}
		m[id] = key
	}
	return m, rows.Err()
}

// LoadKGDatabaseKeyMap 读取 KGMusicV3.db，内存解密并返回
// EncryptionKeyId -> EncryptionKey 映射。全程不落盘。
func LoadKGDatabaseKeyMap(dbPath string) (map[string]string, error) {
	buffer, err := os.ReadFile(dbPath)
	if err != nil {
		return nil, err
	}
	if err := decryptPcDatabase(buffer); err != nil {
		return nil, err
	}
	return extractKeyMapping(buffer)
}

// --- AES-128-CBC (stdlib) ---

func aesCBCDecrypt(cipher, key, iv []byte) ([]byte, error) {
	dst := make([]byte, len(cipher))
	if err := aesCBCDecryptTo(dst, cipher, key, iv); err != nil {
		return nil, err
	}
	return dst, nil
}

func aesCBCDecryptTo(dst, cipher, key, iv []byte) error {
	block, err := aesNewCipher(key)
	if err != nil {
		return err
	}
	if len(cipher)%block.BlockSize() != 0 {
		return errors.New("cipher length is not aligned with block size")
	}
	if len(dst) != len(cipher) {
		return errors.New("decrypt destination size does not match cipher size")
	}
	mode := newCBCDecrypter(block, iv)
	mode.CryptBlocks(dst, cipher)
	return nil
}

type aesBlock interface {
	BlockSize() int
	Encrypt(dst, src []byte)
	Decrypt(dst, src []byte)
}

func aesNewCipher(key []byte) (aesBlock, error)          { return aesNewCipherImpl(key) }
func newCBCDecrypter(b aesBlock, iv []byte) cbcDecrypter { return newCBCDecrypterImpl(b, iv) }

type cbcDecrypter interface{ CryptBlocks(dst, src []byte) }
