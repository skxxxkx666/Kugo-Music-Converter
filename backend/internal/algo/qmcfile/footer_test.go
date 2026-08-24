package qmcfile

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func TestInspectMusicEx(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		mediaMID string
	}{
		{name: "mflac", resource: "C400000012345678.mflac", mediaMID: "003SyntheticMID"},
		{name: "mgg", resource: "C400000087654321.MGG", mediaMID: "MID-\U0001F3B5-test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audio := syntheticAudio(4097)
			footer := makeMusicExFooter(t, tt.mediaMID, tt.resource)
			path := writeFixture(t, append(append([]byte(nil), audio...), footer...))

			info, err := Inspect(path)
			if err != nil {
				t.Fatalf("Inspect() error = %v", err)
			}
			if info.Kind != KindMusicEx || info.Kind.String() != "musicex" {
				t.Fatalf("Kind = %v, want %v", info.Kind, KindMusicEx)
			}
			if info.AudioLen != int64(len(audio)) {
				t.Fatalf("AudioLen = %d, want %d", info.AudioLen, len(audio))
			}
			if info.FooterSize != MusicExFooterSize || info.Version != MusicExVersion {
				t.Fatalf("footer = size %d version %d", info.FooterSize, info.Version)
			}
			if info.Metadata == nil {
				t.Fatal("Metadata is nil")
			}
			want := Metadata{
				SongID:           0x10203040,
				Quality1:         0x11223344,
				Quality2:         0x55667788,
				MediaMID:         tt.mediaMID,
				ResourceFilename: tt.resource,
			}
			if *info.Metadata != want {
				t.Fatalf("Metadata = %#v, want %#v", *info.Metadata, want)
			}
		})
	}
}

func TestInspectMusicExRejectsMalformedFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func([]byte)
		wantErr error
	}{
		{
			name: "footer below minimum",
			mutate: func(footer []byte) {
				binary.LittleEndian.PutUint32(footer[len(footer)-16:], 15)
			},
			wantErr: ErrMalformedFooter,
		},
		{
			name: "footer above maximum",
			mutate: func(footer []byte) {
				binary.LittleEndian.PutUint32(footer[len(footer)-16:], maxFooterSize+1)
			},
			wantErr: ErrMalformedFooter,
		},
		{
			name: "footer exceeds file",
			mutate: func(footer []byte) {
				binary.LittleEndian.PutUint32(footer[len(footer)-16:], 4096)
			},
			wantErr: ErrMalformedFooter,
		},
		{
			name: "footer consumes whole file",
			mutate: func(footer []byte) {
				binary.LittleEndian.PutUint32(footer[len(footer)-16:], uint32(len(footer)+32))
			},
			wantErr: ErrMalformedFooter,
		},
		{
			name: "truncated v1 footer",
			mutate: func(footer []byte) {
				binary.LittleEndian.PutUint32(footer[len(footer)-16:], MusicExFooterSize-1)
			},
			wantErr: ErrUnsupportedFooter,
		},
		{
			name: "unsupported version",
			mutate: func(footer []byte) {
				binary.LittleEndian.PutUint32(footer[len(footer)-12:], MusicExVersion+1)
			},
			wantErr: ErrUnsupportedFooter,
		},
		{
			name: "unpaired utf16 surrogate",
			mutate: func(footer []byte) {
				binary.LittleEndian.PutUint16(footer[0x0C:], 0xD800)
				binary.LittleEndian.PutUint16(footer[0x0E:], 0)
			},
			wantErr: ErrMalformedFooter,
		},
		{
			name: "utf16 data after terminator",
			mutate: func(footer []byte) {
				binary.LittleEndian.PutUint16(footer[0x0C:], 0)
				binary.LittleEndian.PutUint16(footer[0x0E:], 'X')
			},
			wantErr: ErrMalformedFooter,
		},
		{
			name: "utf16 missing terminator",
			mutate: func(footer []byte) {
				for i := 0x0C; i < 0x48; i += 2 {
					binary.LittleEndian.PutUint16(footer[i:], 'A')
				}
			},
			wantErr: ErrMalformedFooter,
		},
		{
			name: "resource extension",
			mutate: func(footer []byte) {
				putFixedUTF16LE(t, footer[0x48:0x8C], "synthetic.mp3")
			},
			wantErr: ErrMalformedFooter,
		},
		{
			name: "resource path",
			mutate: func(footer []byte) {
				putFixedUTF16LE(t, footer[0x48:0x8C], "dir/song.mflac")
			},
			wantErr: ErrMalformedFooter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			footer := makeMusicExFooter(t, "SyntheticMID", "song.mflac")
			tt.mutate(footer)
			path := writeFixture(t, append(syntheticAudio(32), footer...))
			_, err := Inspect(path)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Inspect() error = %v, want errors.Is(%v)", err, tt.wantErr)
			}
		})
	}
}

func TestInspectFooterKinds(t *testing.T) {
	audio := syntheticAudio(73)
	tests := []struct {
		name       string
		kind       Kind
		footer     []byte
		footerSize uint32
	}{
		{
			name:       "QTag",
			kind:       KindQTag,
			footer:     taggedFooter("QTag", []byte("synthetic qtag payload")),
			footerSize: uint32(len("synthetic qtag payload") + 8),
		},
		{
			name:       "STag",
			kind:       KindSTag,
			footer:     taggedFooter("STag", []byte("synthetic stag payload")),
			footerSize: uint32(len("synthetic stag payload") + 8),
		},
		{
			name:       "legacy appended key",
			kind:       KindLegacy,
			footer:     legacyFooter([]byte("synthetic legacy key")),
			footerSize: uint32(len("synthetic legacy key") + 4),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFixture(t, append(append([]byte(nil), audio...), tt.footer...))
			info, err := Inspect(path)
			if err != nil {
				t.Fatalf("Inspect() error = %v", err)
			}
			if info.Kind != tt.kind || info.AudioLen != int64(len(audio)) || info.FooterSize != tt.footerSize {
				t.Fatalf("Info = %#v, want kind=%v audio=%d footer=%d", info, tt.kind, len(audio), tt.footerSize)
			}
			if info.Metadata != nil {
				t.Fatalf("Metadata = %#v, want nil", info.Metadata)
			}
		})
	}
}

func TestInspectLegacyWithoutFooter(t *testing.T) {
	data := make([]byte, 64)
	for i := range data {
		data[i] = 0xFF
	}
	info, err := Inspect(writeFixture(t, data))
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if info.Kind != KindLegacy || info.AudioLen != int64(len(data)) || info.FooterSize != 0 {
		t.Fatalf("Info = %#v", info)
	}
}

func TestInspectRejectsMalformedTaggedFooter(t *testing.T) {
	for _, marker := range []string{"QTag", "STag"} {
		t.Run(marker, func(t *testing.T) {
			footer := make([]byte, 8)
			binary.BigEndian.PutUint32(footer, maxFooterSize)
			copy(footer[4:], marker)
			_, err := Inspect(writeFixture(t, append(syntheticAudio(32), footer...)))
			if !errors.Is(err, ErrMalformedFooter) {
				t.Fatalf("Inspect() error = %v, want ErrMalformedFooter", err)
			}
		})
	}
}

func makeMusicExFooter(t *testing.T, mediaMID, resource string) []byte {
	t.Helper()
	footer := make([]byte, MusicExFooterSize)
	binary.LittleEndian.PutUint32(footer[0x00:0x04], 0x10203040)
	binary.LittleEndian.PutUint32(footer[0x04:0x08], 0x11223344)
	binary.LittleEndian.PutUint32(footer[0x08:0x0C], 0x55667788)
	putFixedUTF16LE(t, footer[0x0C:0x48], mediaMID)
	putFixedUTF16LE(t, footer[0x48:0x8C], resource)
	trailer := footer[len(footer)-16:]
	binary.LittleEndian.PutUint32(trailer[0:4], MusicExFooterSize)
	binary.LittleEndian.PutUint32(trailer[4:8], MusicExVersion)
	copy(trailer[8:], musicExMagic[:])
	return footer
}

func putFixedUTF16LE(t *testing.T, field []byte, value string) {
	t.Helper()
	clear(field)
	units := utf16.Encode([]rune(value))
	if len(units)+1 > len(field)/2 {
		t.Fatalf("UTF-16 fixture %q does not fit in %d bytes", value, len(field))
	}
	for i, unit := range units {
		binary.LittleEndian.PutUint16(field[i*2:], unit)
	}
}

func taggedFooter(marker string, payload []byte) []byte {
	footer := append([]byte(nil), payload...)
	var trailer [8]byte
	binary.BigEndian.PutUint32(trailer[:4], uint32(len(payload)))
	copy(trailer[4:], marker)
	return append(footer, trailer[:]...)
}

func legacyFooter(payload []byte) []byte {
	footer := append([]byte(nil), payload...)
	var length [4]byte
	binary.LittleEndian.PutUint32(length[:], uint32(len(payload)))
	return append(footer, length[:]...)
}

func syntheticAudio(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte((i*37 + 11) % 251)
	}
	return data
}

func writeFixture(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic.mflac")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}
