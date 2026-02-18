package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"kugo-music-converter/internal/algo/kgg"
	"kugo-music-converter/internal/config"
	"kugo-music-converter/internal/logger"
	"kugo-music-converter/internal/utils"

	"go.uber.org/zap"

	common "unlock-music.dev/cli/algo/common"
	"unlock-music.dev/cli/algo/kgm"
	"unlock-music.dev/cli/algo/ncm"
)

type DecryptService struct{ cfg *config.Config }

var noopZapLogger = zap.NewNop()

func NewDecryptService(cfg *config.Config) *DecryptService {
	return &DecryptService{cfg: cfg}
}

// DecryptFileByExt selects a decryptor by extension.
func (s *DecryptService) DecryptFileByExt(inPath string) (outPath string, cleanup func(), err error) {
	ext := strings.ToLower(filepath.Ext(inPath))
	switch ext {
	case ".kgm", ".kgma", ".vpr":
		return s.decryptKgmPureGo(inPath)
	case ".kgg":
		return s.decryptKggPureGo(inPath)
	case ".ncm":
		return s.decryptNcmPureGo(inPath)
	default:
		return "", func() {}, fmt.Errorf("%w: %s", ErrUnsupportedInput, ext)
	}
}

// DecryptFileByExtWithMemKey prefers in-memory key map for .kgg.
func (s *DecryptService) DecryptFileByExtWithMemKey(inPath string, memKey map[string]string) (outPath string, cleanup func(), err error) {
	ext := strings.ToLower(filepath.Ext(inPath))
	if ext == ".kgg" && len(memKey) > 0 {
		return s.decryptKggWithProvider(inPath, kgg.MemoryKeyProvider{Cache: memKey})
	}
	return s.DecryptFileByExt(inPath)
}

func (s *DecryptService) decryptKgmPureGo(inPath string) (outPath string, cleanup func(), err error) {
	in, err := os.Open(inPath)
	if err != nil {
		return "", func() {}, err
	}
	defer in.Close()

	dec := kgm.NewDecoder(&common.DecoderParams{Reader: in})
	if err := dec.Validate(); err != nil {
		return "", func() {}, fmt.Errorf("%w: invalid KGM/KGMA/VPR: %v", ErrDecryptProcess, err)
	}

	outPath = filepath.Join(os.TempDir(), fmt.Sprintf("kgm_dec_%s.bin", utils.RandHex(8)))
	out, e := os.Create(outPath)
	if e != nil {
		return "", func() {}, e
	}
	defer out.Close()

	buf := make([]byte, 64*1024)
	for {
		n, e := dec.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return "", func() {}, werr
			}
		}
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			return "", func() {}, fmt.Errorf("%w: %v", ErrDecryptProcess, e)
		}
	}
	return outPath, func() { _ = os.Remove(outPath) }, nil
}

func (s *DecryptService) decryptNcmPureGo(inPath string) (outPath string, cleanup func(), err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("NCM decoder panic: %v", r)
			outPath = ""
			cleanup = func() {}
			err = fmt.Errorf("%w: ncm decoder panic: %v", ErrDecryptProcess, r)
		}
	}()

	in, err := os.Open(inPath)
	if err != nil {
		return "", func() {}, err
	}
	defer in.Close()

	dec := ncm.NewDecoder(&common.DecoderParams{
		Reader:    in,
		Extension: strings.ToLower(filepath.Ext(inPath)),
		FilePath:  inPath,
		Logger:    noopZapLogger,
	})
	if err := dec.Validate(); err != nil {
		return "", func() {}, fmt.Errorf("%w: invalid NCM: %v", ErrDecryptProcess, err)
	}

	outPath = filepath.Join(os.TempDir(), fmt.Sprintf("ncm_dec_%s.bin", utils.RandHex(8)))
	out, e := os.Create(outPath)
	if e != nil {
		return "", func() {}, e
	}
	defer out.Close()

	buf := make([]byte, 64*1024)
	for {
		n, e := dec.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return "", func() {}, werr
			}
		}
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			return "", func() {}, fmt.Errorf("%w: %v", ErrDecryptProcess, e)
		}
	}
	return outPath, func() { _ = os.Remove(outPath) }, nil
}

// decryptKggPureGo prefers keys discovered from tools/KGMusicV3.db.
func (s *DecryptService) decryptKggPureGo(inPath string) (outPath string, cleanup func(), err error) {
	work := filepath.Dir(inPath)
	provider := kgg.TryKeyProviders("", "", work)
	if provider == nil {
		provider = kgg.TryKeyProviders(filepath.Join("tools", "KGMusicV3.db"), "", work)
	}
	if provider == nil {
		return "", func() {}, fmt.Errorf("%w: KGMusicV3.db or kgg.key not found", ErrMissingKGGKey)
	}

	return s.decryptKggWithProvider(inPath, provider)
}

func (s *DecryptService) decryptKggWithProvider(inPath string, provider kgg.KeyProvider) (outPath string, cleanup func(), err error) {
	f, err := os.Open(inPath)
	if err != nil {
		return "", func() {}, err
	}
	defer f.Close()

	dec, err := kgg.NewDecoder(&kgg.DecoderParams{Reader: f, Path: inPath}, provider)
	if err != nil {
		switch {
		case errors.Is(err, kgg.ErrUnsupportedMode):
			return "", func() {}, fmt.Errorf("%w: %v", ErrUnsupportedInput, err)
		case errors.Is(err, kgg.ErrKeyNotFound):
			return "", func() {}, fmt.Errorf("%w: %v", ErrMissingKGGKey, err)
		default:
			return "", func() {}, fmt.Errorf("%w: %v", ErrDecryptProcess, err)
		}
	}
	defer dec.Close()

	outPath = filepath.Join(os.TempDir(), fmt.Sprintf("kgg_dec_%s.bin", utils.RandHex(8)))
	out, e := os.Create(outPath)
	if e != nil {
		return "", func() {}, e
	}
	defer out.Close()

	buf := make([]byte, 64*1024)
	for {
		n, e := dec.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return "", func() {}, werr
			}
		}
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			if errors.Is(e, kgg.ErrKeyNotFound) {
				return "", func() {}, fmt.Errorf("%w: %v", ErrMissingKGGKey, e)
			}
			return "", func() {}, fmt.Errorf("%w: %v", ErrDecryptProcess, e)
		}
	}
	return outPath, func() { _ = os.Remove(outPath) }, nil
}
