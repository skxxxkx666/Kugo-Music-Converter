package kgg

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// --- 解库结果进程级缓存（F-5005）---
//
// 旧实现 provider 按实例缓存、且每个文件新建 provider，导致批量转换时
// KGMusicV3.db 被反复整库解密。改为按文件路径 + 大小 + mtime 签名做进程级
// 缓存（mtime/size 变化自动失效，用户播放新歌后重载 DB 能即时生效），
// 加载期间持锁，N 个并发任务只解库一次。

type keyMapCacheEntry struct {
	sig string
	m   map[string]string
}

var (
	keyMapCacheMu sync.Mutex
	keyMapCache   = map[string]keyMapCacheEntry{}
)

func fileSignature(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", st.Size(), st.ModTime().UnixNano()), nil
}

// cachedKeyMap 返回只读共享映射；并发 Lookup 仅读取，安全。
func cachedKeyMap(path string, loader func(string) (map[string]string, error)) (map[string]string, error) {
	sig, err := fileSignature(path)
	if err != nil {
		return nil, err
	}
	keyMapCacheMu.Lock()
	defer keyMapCacheMu.Unlock()
	if e, ok := keyMapCache[path]; ok && e.sig == sig {
		return e.m, nil
	}
	m, err := loader(path)
	if err != nil {
		return nil, err
	}
	keyMapCache[path] = keyMapCacheEntry{sig: sig, m: m}
	return m, nil
}

// parseKggKeyFile 解析 kgg.key（格式: <id>$<ekey>\n）。
func parseKggKeyFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	var keyBuilder, valBuilder strings.Builder
	stateKey := true
	for _, ch := range data {
		switch ch {
		case '$':
			stateKey = false
		case '\n':
			k, v := keyBuilder.String(), valBuilder.String()
			if k != "" || v != "" {
				out[k] = v
			}
			keyBuilder.Reset()
			valBuilder.Reset()
			stateKey = true
		case '\r':
			// skip
		default:
			if stateKey {
				keyBuilder.WriteByte(ch)
			} else {
				valBuilder.WriteByte(ch)
			}
		}
	}
	if k, v := keyBuilder.String(), valBuilder.String(); k != "" || v != "" {
		out[k] = v
	}
	return out, nil
}

var (
	ErrFileAccessRequired = errors.New("kgg decoder requires file access")
	ErrUnsupportedMode    = errors.New("unsupported kgg mode")
	ErrKeyNotFound        = errors.New("kgg key not found")
	ErrInvalidKGGHeader   = errors.New("invalid kgg header")
)

// KGG/KGM v5 与 VPR 的 16 字节 magic（与 unlock-music kgm_header.go 一致）。
var (
	kgmMagicHeader = [16]byte{
		0x7C, 0xD5, 0x32, 0xEB, 0x86, 0x02, 0x7F, 0x4B,
		0xA8, 0xAF, 0xA6, 0x8E, 0x0F, 0xFF, 0x99, 0x14,
	}
	vprMagicHeader = [16]byte{
		0x05, 0x28, 0xBC, 0x96, 0xE9, 0xE4, 0x5A, 0x43,
		0x91, 0xAA, 0xBD, 0xD0, 0x7A, 0xF5, 0x36, 0x31,
	}
)

// DecoderParams 与 unlock-music 的 common.DecoderParams 对齐的最小子集
type DecoderParams struct {
	Reader io.Reader
	// For file-based operations we also accept a Path when available
	Path string
}

// Decoder 提供与 kgm/ncm 相同的 Validate/Read 风格接口
type Decoder struct {
	r *os.File
	// header length and start offset of encrypted audio
	headerLen int64
	// qmc2 decryptor
	dec QMC2Base
	// streaming state
	offset int64
}

// NewDecoder 需要文件路径或 *os.File 输入
func NewDecoder(p *DecoderParams, keyProvider KeyProvider) (*Decoder, error) {
	var f *os.File
	switch t := p.Reader.(type) {
	case *os.File:
		f = t
	default:
		if p.Path != "" {
			var err error
			f, err = os.Open(p.Path)
			if err != nil {
				return nil, err
			}
		} else {
			// 缺少随机访问能力，拒绝
			return nil, ErrFileAccessRequired
		}
	}

	d := &Decoder{r: f}
	if err := d.prepare(keyProvider); err != nil {
		_ = f.Close()
		return nil, err
	}
	return d, nil
}

func (d *Decoder) Validate() error { return nil }

func (d *Decoder) Read(p []byte) (int, error) {
	if d.r == nil || d.dec == nil {
		return 0, io.EOF
	}
	// 读取下一块并解密
	if d.offset == 0 {
		if _, err := d.r.Seek(d.headerLen, io.SeekStart); err != nil {
			return 0, err
		}
	}
	n, err := d.r.Read(p)
	if n > 0 {
		d.dec.Decrypt(p[:n], uint64(d.offset))
		d.offset += int64(n)
	}
	if err != nil {
		return n, err
	}
	return n, nil
}

func (d *Decoder) Close() error {
	if d.r != nil {
		return d.r.Close()
	}
	return nil
}

// --- internals ---

func (d *Decoder) prepare(keyProvider KeyProvider) error {
	// 结构化解析头部（对齐 unlock-music kgm_header.go）：
	//   0x00..0x0f magic            (校验 kgm / vpr)
	//   0x10       AudioOffset  u32  -> headerLen
	//   0x14       CryptoVersion u32 -> 仅支持 5
	//   0x18       CryptoSlot    u32
	//   0x1c       CryptoTestData 16B
	//   0x2c       CryptoKey      16B
	//   0x3c+0x08  audioHashLen  u32  (= offset 0x44 / 68)
	//   0x48       audioHash     N B
	if _, err := d.r.Seek(0, io.SeekStart); err != nil {
		return err
	}
	var fixed [0x44]byte
	if _, err := io.ReadFull(d.r, fixed[:]); err != nil {
		return fmt.Errorf("%w: read header: %v", ErrInvalidKGGHeader, err)
	}

	var magic [16]byte
	copy(magic[:], fixed[0:16])
	if magic != kgmMagicHeader && magic != vprMagicHeader {
		return fmt.Errorf("%w: magic header not matched", ErrInvalidKGGHeader)
	}

	le32 := func(b []byte) uint32 {
		return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	}
	d.headerLen = int64(le32(fixed[0x10:0x14]))
	cryptoVersion := le32(fixed[0x14:0x18])
	if cryptoVersion != 5 {
		return fmt.Errorf("%w: %d", ErrUnsupportedMode, cryptoVersion)
	}

	// audioHashLen @ 0x44，audioHash 紧随其后
	var b4 [4]byte
	if _, err := io.ReadFull(d.r, b4[:]); err != nil {
		return fmt.Errorf("%w: read audio hash length: %v", ErrInvalidKGGHeader, err)
	}
	hashLen := int(le32(b4[:]))
	if hashLen <= 0 || hashLen > 256 {
		return fmt.Errorf("%w: implausible audio hash length %d", ErrInvalidKGGHeader, hashLen)
	}
	audioHash := make([]byte, hashLen)
	if _, err := io.ReadFull(d.r, audioHash); err != nil {
		return fmt.Errorf("%w: read audio hash: %v", ErrInvalidKGGHeader, err)
	}

	// find ekey by audio hash
	ekey, err := keyProvider.Lookup(string(audioHash))
	if err != nil {
		return err
	}
	q, err := CreateQMC2(ekey)
	if err != nil {
		return err
	}
	d.dec = q
	return nil
}

// --- Key Provider ---

// KeyProvider 从 kgg.key 或 KGMusicV3.db 提供 ekey
type KeyProvider interface {
	Lookup(audioHash string) (string, error)
}

// MemoryKeyProvider 运行时内存中的 key 映射
type MemoryKeyProvider struct{ Cache map[string]string }

func (m MemoryKeyProvider) Lookup(audioHash string) (string, error) {
	if m.Cache == nil {
		return "", fmt.Errorf("%w: %s", ErrKeyNotFound, audioHash)
	}
	if v, ok := m.Cache[audioHash]; ok {
		return v, nil
	}
	return "", fmt.Errorf("%w: %s", ErrKeyNotFound, audioHash)
}

// CombinedProvider 依次查询多个 provider
type CombinedProvider struct{ providers []KeyProvider }

func (c CombinedProvider) Lookup(audioHash string) (string, error) {
	for _, p := range c.providers {
		if p == nil {
			continue
		}
		if v, err := p.Lookup(audioHash); err == nil {
			return v, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrKeyNotFound, audioHash)
}

// FileKeyMapProvider 解析 kgg.key（格式: <id>$<ekey>\n）
type FileKeyMapProvider struct {
	path  string
	cache map[string]string
}

func NewFileKeyMapProvider(path string) *FileKeyMapProvider {
	return &FileKeyMapProvider{path: path, cache: map[string]string{}}
}

func (p *FileKeyMapProvider) ensureLoaded() error {
	if len(p.cache) > 0 {
		return nil
	}
	m, err := cachedKeyMap(p.path, parseKggKeyFile)
	if err != nil {
		return err
	}
	p.cache = m
	return nil
}

func (p *FileKeyMapProvider) Lookup(audioHash string) (string, error) {
	if err := p.ensureLoaded(); err != nil {
		return "", err
	}
	if v, ok := p.cache[audioHash]; ok {
		return v, nil
	}
	return "", fmt.Errorf("%w: %s", ErrKeyNotFound, audioHash)
}

// DBKeyProvider 通过解密 KGMusicV3.db 生成 KeyMap
type DBKeyProvider struct {
	dbPath string
	cache  map[string]string
}

func NewDBKeyProvider(path string) *DBKeyProvider {
	return &DBKeyProvider{dbPath: path, cache: map[string]string{}}
}

func (p *DBKeyProvider) ensureLoaded() error {
	if len(p.cache) > 0 {
		return nil
	}
	// 内存解密 KGMusicV3.db，结果按 dbPath 进程级缓存（见 cachedKeyMap）。
	m, err := cachedKeyMap(p.dbPath, LoadKGDatabaseKeyMap)
	if err != nil {
		return err
	}
	p.cache = m
	return nil
}

func (p *DBKeyProvider) Lookup(audioHash string) (string, error) {
	if err := p.ensureLoaded(); err != nil {
		return "", err
	}
	if v, ok := p.cache[audioHash]; ok {
		return v, nil
	}
	return "", fmt.Errorf("%w: %s", ErrKeyNotFound, audioHash)
}

// Helper: TryKeyProviders tries a list of providers
func TryKeyProviders(dbPath, keyPath string, workDir string) KeyProvider {
	var ps []KeyProvider
	if keyPath != "" {
		ps = append(ps, NewFileKeyMapProvider(keyPath))
	}
	if dbPath != "" {
		ps = append(ps, NewDBKeyProvider(dbPath))
	}
	// 自动发现 tools 目录（先 key 后 db）
	for _, base := range []string{workDir, "."} {
		cand := filepath.Join(base, "tools", "kgg.key")
		if _, err := os.Stat(cand); err == nil {
			ps = append(ps, NewFileKeyMapProvider(cand))
			break
		}
	}
	for _, base := range []string{workDir, "."} {
		cand := filepath.Join(base, "tools", "KGMusicV3.db")
		if _, err := os.Stat(cand); err == nil {
			ps = append(ps, NewDBKeyProvider(cand))
			break
		}
	}
	if len(ps) == 0 {
		return nil
	}
	if len(ps) == 1 {
		return ps[0]
	}
	return CombinedProvider{providers: ps}
}
