package kgg

import (
	"crypto/md5"
	"database/sql"
	"errors"
	"io"
	"os"

	_ "modernc.org/sqlite"
)

// DecryptKGDatabaseToFile 将 KGMusicV3.db 解密为标准 SQLite 文件，返回临时路径
func DecryptKGDatabaseToFile(dbPath string) (string, func(), error) {
	f, err := os.Open(dbPath)
	if err != nil {
		return "", func() {}, err
	}
	defer f.Close()

	const pageSize = 1024
	// 读取总大小
	info, err := f.Stat()
	if err != nil {
		return "", func() {}, err
	}
	if info.Size()%pageSize != 0 {
		return "", func() {}, errors.New("invalid kg db size")
	}
	pages := int(info.Size() / pageSize)

	tmpFile, err := os.CreateTemp("", "kgdb_dec_*.sqlite")
	if err != nil {
		return "", func() {}, err
	}
	tmp := tmpFile.Name()
	out := tmpFile

	buf := make([]byte, pageSize)
	for page := 1; page <= pages; page++ {
		if _, err := io.ReadFull(f, buf); err != nil {
			out.Close()
			os.Remove(tmp)
			return "", func() {}, err
		}
		var key, iv [16]byte
		derivePageKey(&key, &iv, defaultMasterKey[:], uint32(page))
		if page == 1 {
			// Detect unencrypted
			if isSQLiteHeader(buf) {
				// copy first page then rest
				if _, err := out.Write(buf); err != nil {
					out.Close()
					os.Remove(tmp)
					return "", func() {}, err
				}
				if _, err := io.Copy(out, f); err != nil {
					out.Close()
					os.Remove(tmp)
					return "", func() {}, err
				}
				out.Close()
				return tmp, func() { _ = os.Remove(tmp) }, nil
			}
			if !isValidPage1Header(buf) {
				out.Close()
				os.Remove(tmp)
				return "", func() {}, errors.New("invalid page1 header")
			}
			// swap and decrypt from offset 16
			backup := make([]byte, 8)
			copy(backup, buf[16:24])
			copy(buf[16:], buf[8:16])
			plain := aesCBCDecrypt(buf[16:], key[:], iv[:])
			// write header + plain
			if _, err := out.Write(sqliteHeader); err != nil {
				out.Close()
				os.Remove(tmp)
				return "", func() {}, err
			}
			if _, err := out.Write(plain); err != nil {
				out.Close()
				os.Remove(tmp)
				return "", func() {}, err
			}
			if copy(backup, plain[:8]); string(backup) == string(plain[:8]) { /* ok */
			}
		} else {
			plain := aesCBCDecrypt(buf, key[:], iv[:])
			if _, err := out.Write(plain); err != nil {
				out.Close()
				os.Remove(tmp)
				return "", func() {}, err
			}
		}
	}
	out.Close()
	return tmp, func() { _ = os.Remove(tmp) }, nil
}

// ReadShareFileItems 读取解密后 sqlite 的映射表
func ReadShareFileItems(sqlitePath string) (map[string]string, error) {
	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query("SELECT EncryptionKeyId, EncryptionKey FROM ShareFileItems WHERE EncryptionKeyId IS NOT NULL AND EncryptionKeyId != '' AND EncryptionKey IS NOT NULL AND EncryptionKey != ''")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			return nil, err
		}
		m[id] = key
	}
	return m, nil
}

// --- Helpers (crypto) ---

var sqliteHeader = []byte("SQLite format 3\x00")
var defaultMasterKey = [16]byte{0x1d, 0x61, 0x31, 0x45, 0xb2, 0x47, 0xbf, 0x7f, 0x3d, 0x18, 0x96, 0x72, 0x14, 0x4f, 0xe4, 0xbf}

func isSQLiteHeader(b []byte) bool { return len(b) >= 16 && string(b[:16]) == string(sqliteHeader) }

func isValidPage1Header(page1 []byte) bool {
	if len(page1) < 24 {
		return false
	}
	o10 := uint32(page1[16]) | uint32(page1[17])<<8 | uint32(page1[18])<<16 | uint32(page1[19])<<24
	o14 := uint32(page1[20]) | uint32(page1[21])<<8 | uint32(page1[22])<<16 | uint32(page1[23])<<24
	v6 := ((o10 & 0xFF) << 8) | ((o10 & 0xFF00) << 16)
	return (o14 == 0x20204000) && (v6-0x200 <= 0xFE00) && ((v6 & (v6 - 1)) == 0)
}

func derivePageKey(aesKey, aesIV *[16]byte, master []byte, pageNo uint32) {
	buf := make([]byte, 0x18)
	copy(buf[:16], master)
	buf[16] = byte(pageNo)
	buf[17] = byte(pageNo >> 8)
	buf[18] = byte(pageNo >> 16)
	buf[19] = byte(pageNo >> 24)
	magic := uint32(0x546C4173)
	buf[20] = byte(magic)
	buf[21] = byte(magic >> 8)
	buf[22] = byte(magic >> 16)
	buf[23] = byte(magic >> 24)
	sum := md5.Sum(buf)
	copy(aesKey[:], sum[:])
	// IV
	ebx := pageNo + 1
	for i := 0; i < 16; i += 4 {
		divisor := uint32(0xCE26)
		quotient := ebx / divisor
		eax := 0x7FFFFF07 * quotient
		ecx := 0x9EF4*ebx - eax
		if ecx&0x80000000 != 0 {
			ecx += 0x7FFFFF07
		}
		ebx = ecx
		buf[i] = byte(ebx)
		buf[i+1] = byte(ebx >> 8)
		buf[i+2] = byte(ebx >> 16)
		buf[i+3] = byte(ebx >> 24)
	}
	sum = md5.Sum(buf[:16])
	copy(aesIV[:], sum[:])
}

// AES-128-CBC decrypt using Go stdlib crypto
func aesCBCDecrypt(cipher, key, iv []byte) []byte {
	// use crypto/aes + crypto/cipher
	block, err := aesNewCipher(key)
	if err != nil {
		return nil
	}
	if len(cipher)%block.BlockSize() != 0 {
		return nil
	}
	dst := make([]byte, len(cipher))
	mode := newCBCDecrypter(block, iv)
	mode.CryptBlocks(dst, cipher)
	return dst
}

// small local wrappers to avoid importing directly in many places
// (helps keep editor diffs minimal)
type aesBlock interface {
	BlockSize() int
	Encrypt(dst, src []byte)
	Decrypt(dst, src []byte)
}

func aesNewCipher(key []byte) (aesBlock, error)          { return aesNewCipherImpl(key) }
func newCBCDecrypter(b aesBlock, iv []byte) cbcDecrypter { return newCBCDecrypterImpl(b, iv) }

type cbcDecrypter interface{ CryptBlocks(dst, src []byte) }
