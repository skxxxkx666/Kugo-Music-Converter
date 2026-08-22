package qmcfile

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf16"
)

const (
	minFooterSize = uint32(16)
	maxFooterSize = uint32(64 * 1024)

	musicExMetadataEnd = uint32(0x8C)
)

var musicExMagic = [8]byte{'m', 'u', 's', 'i', 'c', 'e', 'x', 0}

// Inspect opens path, validates its trailing QMC footer, and returns only
// structured metadata. It never returns raw footer data.
func Inspect(path string) (Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return Info{}, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return Info{}, err
	}
	return inspectFile(f, st.Size())
}

func inspectFile(r io.ReaderAt, fileSize int64) (Info, error) {
	if fileSize < 4 {
		return Info{}, malformed("file is too small: %d bytes", fileSize)
	}

	if fileSize >= 16 {
		var trailer [16]byte
		if _, err := r.ReadAt(trailer[:], fileSize-int64(len(trailer))); err != nil {
			return Info{}, malformed("read trailer: %v", err)
		}
		if string(trailer[8:]) == string(musicExMagic[:]) {
			return inspectMusicEx(r, fileSize, trailer)
		}
	}

	var tail [8]byte
	tailLen := int64(len(tail))
	if fileSize < tailLen {
		tailLen = fileSize
	}
	if _, err := r.ReadAt(tail[len(tail)-int(tailLen):], fileSize-tailLen); err != nil {
		return Info{}, malformed("read footer marker: %v", err)
	}
	if tailLen == int64(len(tail)) {
		marker := string(tail[4:])
		switch marker {
		case "QTag":
			return inspectTagged(fileSize, tail[:4], KindQTag)
		case "STag":
			return inspectTagged(fileSize, tail[:4], KindSTag)
		}
	}

	return inspectLegacy(fileSize, tail[len(tail)-4:])
}

func inspectMusicEx(r io.ReaderAt, fileSize int64, trailer [16]byte) (Info, error) {
	footerSize := binary.LittleEndian.Uint32(trailer[0:4])
	version := binary.LittleEndian.Uint32(trailer[4:8])
	if err := validateFooterSize(fileSize, footerSize); err != nil {
		return Info{}, err
	}
	if footerSize != MusicExFooterSize {
		return Info{}, fmt.Errorf("%w: musicex v1 footer size %d", ErrUnsupportedFooter, footerSize)
	}
	if footerSize < musicExMetadataEnd+uint32(len(trailer)) {
		return Info{}, malformed("musicex footer is too small for fixed fields: %d", footerSize)
	}
	if version != MusicExVersion {
		return Info{}, fmt.Errorf("%w: musicex version %d", ErrUnsupportedFooter, version)
	}

	footer := make([]byte, footerSize)
	if _, err := r.ReadAt(footer, fileSize-int64(footerSize)); err != nil {
		return Info{}, malformed("read musicex footer: %v", err)
	}
	trailerOffset := len(footer) - len(trailer)
	if string(footer[trailerOffset+8:]) != string(musicExMagic[:]) ||
		binary.LittleEndian.Uint32(footer[trailerOffset:trailerOffset+4]) != footerSize ||
		binary.LittleEndian.Uint32(footer[trailerOffset+4:trailerOffset+8]) != version {
		return Info{}, malformed("musicex trailer changed while reading")
	}

	mediaMID, err := decodeFixedUTF16LE(footer[0x0C:0x48], "media_mid")
	if err != nil {
		return Info{}, err
	}
	resourceFilename, err := decodeFixedUTF16LE(footer[0x48:0x8C], "resource filename")
	if err != nil {
		return Info{}, err
	}
	if mediaMID == "" {
		return Info{}, malformed("media_mid is empty")
	}
	if err := validateResourceFilename(resourceFilename); err != nil {
		return Info{}, err
	}

	return Info{
		Kind:       KindMusicEx,
		AudioLen:   fileSize - int64(footerSize),
		FooterSize: footerSize,
		Version:    version,
		Metadata: &Metadata{
			SongID:           binary.LittleEndian.Uint32(footer[0x00:0x04]),
			Quality1:         binary.LittleEndian.Uint32(footer[0x04:0x08]),
			Quality2:         binary.LittleEndian.Uint32(footer[0x08:0x0C]),
			MediaMID:         mediaMID,
			ResourceFilename: resourceFilename,
		},
	}, nil
}

func inspectTagged(fileSize int64, lengthBytes []byte, kind Kind) (Info, error) {
	payloadSize := binary.BigEndian.Uint32(lengthBytes)
	footerSize, ok := totalFooterSize(payloadSize, 8)
	if !ok || !validFooterSize(fileSize, footerSize) {
		return Info{}, malformed("%s footer has invalid payload length %d", kind, payloadSize)
	}
	return Info{
		Kind:       kind,
		AudioLen:   fileSize - int64(footerSize),
		FooterSize: footerSize,
	}, nil
}

func inspectLegacy(fileSize int64, lengthBytes []byte) (Info, error) {
	payloadSize := binary.LittleEndian.Uint32(lengthBytes)
	footerSize, ok := totalFooterSize(payloadSize, 4)
	if !ok || !validFooterSize(fileSize, footerSize) {
		// Static legacy QMC has no appended key footer. Its entire file is audio.
		return Info{Kind: KindLegacy, AudioLen: fileSize}, nil
	}
	return Info{
		Kind:       KindLegacy,
		AudioLen:   fileSize - int64(footerSize),
		FooterSize: footerSize,
	}, nil
}

func totalFooterSize(payloadSize, trailerSize uint32) (uint32, bool) {
	if payloadSize > maxFooterSize-trailerSize {
		return 0, false
	}
	return payloadSize + trailerSize, true
}

func validateFooterSize(fileSize int64, footerSize uint32) error {
	if !validFooterSize(fileSize, footerSize) {
		return malformed("footer size %d is outside [%d,%d] or leaves no audio in file size %d", footerSize, minFooterSize, maxFooterSize, fileSize)
	}
	return nil
}

func validFooterSize(fileSize int64, footerSize uint32) bool {
	return footerSize >= minFooterSize && footerSize <= maxFooterSize && int64(footerSize) < fileSize
}

func decodeFixedUTF16LE(field []byte, name string) (string, error) {
	if len(field)%2 != 0 {
		return "", malformed("%s UTF-16LE field has odd byte length", name)
	}

	units := make([]uint16, len(field)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(field[i*2 : i*2+2])
	}

	end := -1
	for i, unit := range units {
		if unit == 0 {
			end = i
			break
		}
	}
	if end < 0 {
		return "", malformed("%s UTF-16LE field is not NUL-terminated", name)
	}
	for _, unit := range units[end+1:] {
		if unit != 0 {
			return "", malformed("%s UTF-16LE field has nonzero data after terminator", name)
		}
	}

	for i := 0; i < end; i++ {
		u := units[i]
		switch {
		case u >= 0xD800 && u <= 0xDBFF:
			if i+1 >= end || units[i+1] < 0xDC00 || units[i+1] > 0xDFFF {
				return "", malformed("%s UTF-16LE field has an unpaired high surrogate", name)
			}
			i++
		case u >= 0xDC00 && u <= 0xDFFF:
			return "", malformed("%s UTF-16LE field has an unpaired low surrogate", name)
		}
	}

	return string(utf16.Decode(units[:end])), nil
}

func validateResourceFilename(name string) error {
	if name == "" {
		return malformed("resource filename is empty")
	}
	if strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return malformed("resource filename is not a base filename")
	}
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".mflac") && !strings.HasSuffix(lower, ".mgg") {
		return malformed("resource filename has unsupported extension")
	}
	return nil
}

func malformed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrMalformedFooter, fmt.Sprintf(format, args...))
}
