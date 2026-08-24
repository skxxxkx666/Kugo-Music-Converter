package qmcfile

import (
	"fmt"
	"io"
	"os"
	"strings"

	common "unlock-music.dev/cli/algo/common"
	"unlock-music.dev/cli/algo/qmc"
)

// Open inspects a musicex QMC file and returns a stream that decrypts exactly
// Info.AudioLen bytes. The returned reader owns and closes the underlying file.
func Open(path, ekey string) (io.ReadCloser, Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Info{}, err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = f.Close()
		}
	}()

	st, err := f.Stat()
	if err != nil {
		return nil, Info{}, err
	}
	info, err := inspectFile(f, st.Size())
	if err != nil {
		return nil, Info{}, err
	}
	if info.Kind != KindMusicEx {
		return nil, info, fmt.Errorf("%w: %s", ErrUnsupportedFooter, info.Kind)
	}
	if strings.TrimSpace(ekey) == "" {
		return nil, info, ErrMissingEKey
	}

	decryptor, err := qmc.NewQmcCipherDecoderFromEKey([]byte(ekey))
	if err != nil {
		return nil, info, fmt.Errorf("%w: %v", ErrInvalidEKey, err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, info, err
	}

	closeOnError = false
	return &decryptReader{
		file:      f,
		remaining: info.AudioLen,
		decryptor: decryptor,
	}, info, nil
}

type decryptReader struct {
	file      *os.File
	remaining int64
	offset    int
	decryptor common.StreamDecoder
	closed    bool
}

func (r *decryptReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, os.ErrClosed
	}
	if len(p) == 0 {
		return 0, nil
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}

	readBuf := p
	if int64(len(readBuf)) > r.remaining {
		readBuf = readBuf[:r.remaining]
	}
	n, err := r.file.Read(readBuf)
	if n > 0 {
		r.decryptor.Decrypt(readBuf[:n], r.offset)
		r.offset += n
		r.remaining -= int64(n)
	}
	if err == io.EOF && r.remaining > 0 {
		return n, io.ErrUnexpectedEOF
	}
	if err != nil {
		return n, err
	}
	if n == 0 {
		return 0, io.ErrNoProgress
	}
	return n, nil
}

func (r *decryptReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.file.Close()
}
