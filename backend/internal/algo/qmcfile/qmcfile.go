// Package qmcfile inspects modern QMC containers and exposes bounded,
// streaming decryption for musicex files.
package qmcfile

import "errors"

const (
	// MusicExFooterSize is the size used by the current musicex format.
	MusicExFooterSize uint32 = 0xC0
	// MusicExVersion is the currently supported musicex footer version.
	MusicExVersion uint32 = 1
)

var (
	// ErrMalformedFooter reports a recognized footer whose bounds or fields are invalid.
	ErrMalformedFooter = errors.New("malformed qmc footer")
	// ErrUnsupportedFooter reports a valid footer variant this package cannot decrypt.
	ErrUnsupportedFooter = errors.New("unsupported qmc footer")
	// ErrMissingEKey reports that musicex decryption was requested without an ekey.
	ErrMissingEKey = errors.New("missing qmc ekey")
	// ErrInvalidEKey reports that kgg could not construct a decryptor from the ekey.
	ErrInvalidEKey = errors.New("invalid qmc ekey")
)

// Kind identifies the QMC footer/container generation.
type Kind uint8

const (
	KindUnknown Kind = iota
	KindLegacy
	KindQTag
	KindSTag
	KindMusicEx
)

func (k Kind) String() string {
	switch k {
	case KindLegacy:
		return "legacy"
	case KindQTag:
		return "QTag"
	case KindSTag:
		return "STag"
	case KindMusicEx:
		return "musicex"
	default:
		return "unknown"
	}
}

// Metadata contains the structured, non-secret fields in a musicex footer.
type Metadata struct {
	SongID           uint32
	Quality1         uint32
	Quality2         uint32
	MediaMID         string
	ResourceFilename string
}

// Info describes a QMC file without retaining or exposing raw footer bytes.
// Metadata is non-nil only for KindMusicEx. FooterSize is the total number of
// non-audio bytes at the end of the file.
type Info struct {
	Kind       Kind
	AudioLen   int64
	FooterSize uint32
	Version    uint32
	Metadata   *Metadata
}
