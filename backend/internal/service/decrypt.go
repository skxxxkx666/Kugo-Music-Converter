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

	"go.uber.org/zap"

	common "unlock-music.dev/cli/algo/common"
	"unlock-music.dev/cli/algo/kgm"
	"unlock-music.dev/cli/algo/kwm"
	"unlock-music.dev/cli/algo/ncm"
	"unlock-music.dev/cli/algo/qmc"
)

type DecryptService struct{ cfg *config.Config }

var noopZapLogger = zap.NewNop()

func NewDecryptService(cfg *config.Config) *DecryptService {
	return &DecryptService{cfg: cfg}
}

type stackedReadCloser struct {
	reader  io.Reader
	closers []io.Closer
	closed  bool
}

func (s *stackedReadCloser) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

func (s *stackedReadCloser) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	var firstErr error
	for i := len(s.closers) - 1; i >= 0; i-- {
		if s.closers[i] == nil {
			continue
		}
		if err := s.closers[i].Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func newStackedReadCloser(reader io.Reader, closers ...io.Closer) io.ReadCloser {
	return &stackedReadCloser{
		reader:  reader,
		closers: closers,
	}
}

type panicSafeReadCloser struct {
	kind  string
	inner io.ReadCloser
}

func (p *panicSafeReadCloser) Read(buf []byte) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("%s decoder panic: %v", strings.ToUpper(p.kind), r)
			err = fmt.Errorf("%w: %s decoder panic: %v", ErrDecryptProcess, p.kind, r)
		}
	}()
	return p.inner.Read(buf)
}

func (p *panicSafeReadCloser) Close() (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("%s decoder close panic: %v", strings.ToUpper(p.kind), r)
			err = fmt.Errorf("%w: %s decoder close panic: %v", ErrDecryptProcess, p.kind, r)
		}
	}()
	return p.inner.Close()
}

func withPanicSafeReader(kind string, inner io.ReadCloser) io.ReadCloser {
	return &panicSafeReadCloser{
		kind:  kind,
		inner: inner,
	}
}

func buildDecryptReader(kind string, build func() (io.ReadCloser, error)) (out io.ReadCloser, err error) {
	defer func() {
		if r := recover(); r != nil {
			if out != nil {
				_ = out.Close()
				out = nil
			}
			logger.Errorf("%s decoder panic: %v", strings.ToUpper(kind), r)
			err = fmt.Errorf("%w: %s decoder panic: %v", ErrDecryptProcess, kind, r)
		}
	}()

	stream, err := build()
	if err != nil {
		return nil, err
	}
	return withPanicSafeReader(kind, stream), nil
}

func recoverDecryptPanic(kind string, outPath *string, cleanup *func(), err *error) {
	if r := recover(); r != nil {
		logger.Errorf("%s decoder panic: %v", strings.ToUpper(kind), r)
		if outPath != nil && *outPath != "" {
			_ = os.Remove(*outPath)
		}
		if cleanup != nil {
			*cleanup = func() {}
		}
		if outPath != nil {
			*outPath = ""
		}
		if err != nil {
			*err = fmt.Errorf("%w: %s decoder panic: %v", ErrDecryptProcess, kind, r)
		}
	}
}

// DecryptFileByExt selects a decryptor by extension.
func (s *DecryptService) DecryptFileByExt(inPath string) (io.ReadCloser, error) {
	ext := strings.ToLower(filepath.Ext(inPath))
	switch ext {
	case ".kgm", ".kgma", ".vpr":
		return s.decryptKgmPureGo(inPath)
	case ".kgg":
		return s.decryptKggPureGo(inPath)
	case ".ncm":
		return s.decryptNcmPureGo(inPath)
	case ".kwm":
		return s.decryptKwmPureGo(inPath)
	case ".qmc0", ".qmc2", ".qmc3", ".qmc4", ".qmc6", ".qmc8", ".qmcflac", ".qmcogg", ".tkm":
		return s.decryptQmcPureGo(inPath)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedInput, ext)
	}
}

// DecryptFileByExtWithMemKey prefers in-memory key map for .kgg.
func (s *DecryptService) DecryptFileByExtWithMemKey(inPath string, memKey map[string]string) (io.ReadCloser, error) {
	ext := strings.ToLower(filepath.Ext(inPath))
	if ext == ".kgg" && len(memKey) > 0 {
		return s.decryptKggWithProvider(inPath, kgg.MemoryKeyProvider{Cache: memKey})
	}
	return s.DecryptFileByExt(inPath)
}

func (s *DecryptService) decryptKgmPureGo(inPath string) (io.ReadCloser, error) {
	return buildDecryptReader("kgm", func() (io.ReadCloser, error) {
		in, err := os.Open(inPath)
		if err != nil {
			return nil, err
		}

		dec := kgm.NewDecoder(&common.DecoderParams{Reader: in})
		if err := dec.Validate(); err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("%w: invalid KGM/KGMA/VPR: %v", ErrDecryptProcess, err)
		}
		return newStackedReadCloser(dec, in), nil
	})
}

func (s *DecryptService) decryptNcmPureGo(inPath string) (io.ReadCloser, error) {
	return buildDecryptReader("ncm", func() (io.ReadCloser, error) {
		in, err := os.Open(inPath)
		if err != nil {
			return nil, err
		}

		dec := ncm.NewDecoder(&common.DecoderParams{
			Reader:    in,
			Extension: strings.ToLower(filepath.Ext(inPath)),
			FilePath:  inPath,
			Logger:    noopZapLogger,
		})
		if err := dec.Validate(); err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("%w: invalid NCM: %v", ErrDecryptProcess, err)
		}
		return newStackedReadCloser(dec, in), nil
	})
}

func (s *DecryptService) decryptKwmPureGo(inPath string) (io.ReadCloser, error) {
	return buildDecryptReader("kwm", func() (io.ReadCloser, error) {
		in, err := os.Open(inPath)
		if err != nil {
			return nil, err
		}

		dec := kwm.NewDecoder(&common.DecoderParams{
			Reader:    in,
			Extension: ".kwm",
			FilePath:  inPath,
			Logger:    noopZapLogger,
		})
		if err := dec.Validate(); err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("%w: invalid KWM: %v", ErrDecryptProcess, err)
		}
		return newStackedReadCloser(dec, in), nil
	})
}

func (s *DecryptService) decryptQmcPureGo(inPath string) (io.ReadCloser, error) {
	return buildDecryptReader("qmc", func() (io.ReadCloser, error) {
		in, err := os.Open(inPath)
		if err != nil {
			return nil, err
		}

		dec := qmc.NewDecoder(&common.DecoderParams{
			Reader:    in,
			Extension: strings.ToLower(filepath.Ext(inPath)),
			FilePath:  inPath,
			Logger:    noopZapLogger,
		})
		if err := dec.Validate(); err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("%w: invalid QMC: %v", ErrDecryptProcess, err)
		}
		return newStackedReadCloser(dec, in), nil
	})
}

// decryptKggPureGo prefers keys discovered from tools/KGMusicV3.db.
func (s *DecryptService) decryptKggPureGo(inPath string) (io.ReadCloser, error) {
	work := filepath.Dir(inPath)
	provider := kgg.TryKeyProviders("", "", work)
	if provider == nil {
		return nil, fmt.Errorf("%w: KGMusicV3.db or kgg.key not found", ErrMissingKGGKey)
	}

	return s.decryptKggWithProvider(inPath, provider)
}

func (s *DecryptService) decryptKggWithProvider(inPath string, provider kgg.KeyProvider) (io.ReadCloser, error) {
	return buildDecryptReader("kgg", func() (io.ReadCloser, error) {
		f, err := os.Open(inPath)
		if err != nil {
			return nil, err
		}

		dec, err := kgg.NewDecoder(&kgg.DecoderParams{Reader: f, Path: inPath}, provider)
		if err != nil {
			_ = f.Close()
			switch {
			case errors.Is(err, kgg.ErrUnsupportedMode), errors.Is(err, kgg.ErrInvalidKGGHeader):
				return nil, fmt.Errorf("%w: %v", ErrUnsupportedInput, err)
			case errors.Is(err, kgg.ErrKeyNotFound):
				return nil, fmt.Errorf("%w: %v", ErrMissingKGGKey, err)
			default:
				return nil, fmt.Errorf("%w: %v", ErrDecryptProcess, err)
			}
		}
		// kgg.Decoder owns the underlying file and closes it.
		return dec, nil
	})
}
