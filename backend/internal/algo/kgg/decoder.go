package kgg

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	ErrFileAccessRequired = errors.New("kgg decoder requires file access")
	ErrUnsupportedMode    = errors.New("unsupported kgg mode")
	ErrKeyNotFound        = errors.New("kgg key not found")
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
	// header length at offset 16, mode at 20
	if _, err := d.r.Seek(16, io.SeekStart); err != nil {
		return err
	}
	var hdr [8]byte
	if _, err := io.ReadFull(d.r, hdr[:]); err != nil {
		return err
	}
	d.headerLen = int64(uint32(hdr[0]) | uint32(hdr[1])<<8 | uint32(hdr[2])<<16 | uint32(hdr[3])<<24)
	mode := uint32(hdr[4]) | uint32(hdr[5])<<8 | uint32(hdr[6])<<16 | uint32(hdr[7])<<24
	if mode != 5 {
		return fmt.Errorf("%w: %d", ErrUnsupportedMode, mode)
	}

	// audio_hash at offset 68: len(uint32 LE) + bytes
	if _, err := d.r.Seek(68, io.SeekStart); err != nil {
		return err
	}
	var b4 [4]byte
	if _, err := io.ReadFull(d.r, b4[:]); err != nil {
		return err
	}
	hashLen := int(uint32(b4[0]) | uint32(b4[1])<<8 | uint32(b4[2])<<16 | uint32(b4[3])<<24)
	audioHash := make([]byte, hashLen)
	if _, err := io.ReadFull(d.r, audioHash); err != nil {
		return err
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
	f, err := os.Open(p.path)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	var key string
	var val string
	stateKey := true
	for _, ch := range string(data) {
		switch ch {
		case '$':
			stateKey = false
		case '\n':
			if key != "" || val != "" {
				p.cache[key] = val
			}
			key, val, stateKey = "", "", true
		case '\r':
			// skip
		default:
			if stateKey {
				key += string(ch)
			} else {
				val += string(ch)
			}
		}
	}
	if key != "" || val != "" {
		p.cache[key] = val
	}
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
	// 解密数据库到临时文件并读取映射
	tmp, cleanup, err := DecryptKGDatabaseToFile(p.dbPath)
	if err != nil {
		return err
	}
	defer cleanup()
	m, err := ReadShareFileItems(tmp)
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
