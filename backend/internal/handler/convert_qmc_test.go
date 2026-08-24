package handler

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"kugo-music-converter/internal/qmckey"
	"kugo-music-converter/internal/service"
)

type fakeQMCBatchResolver struct {
	resources []qmckey.Resource
	results   []qmckey.BatchResult
}

func (f *fakeQMCBatchResolver) Resolve(ctx context.Context, resource qmckey.Resource) (string, error) {
	results := f.ResolveBatch(ctx, []qmckey.Resource{resource})
	if len(results) == 0 {
		return "", qmckey.ErrProtocol
	}
	return results[0].EKey, results[0].Err
}

func (f *fakeQMCBatchResolver) ResolveBatch(_ context.Context, resources []qmckey.Resource) []qmckey.BatchResult {
	f.resources = append(f.resources, resources...)
	if f.results != nil {
		return f.results
	}
	results := make([]qmckey.BatchResult, len(resources))
	for index, resource := range resources {
		results[index] = qmckey.BatchResult{Resource: resource, EKey: strings.Repeat("e", 32)}
	}
	return results
}

func TestResolveQMCBatchKeysDeduplicatesResources(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "one.mflac"), filepath.Join(root, "two.mflac")}
	for _, path := range paths {
		fixture := append([]byte("encrypted"), testMusicExFooter(t, "001SameMID", "F0M000Same.mflac")...)
		if err := os.WriteFile(path, fixture, 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}
	resolver := &fakeQMCBatchResolver{}
	handler := &ConvertHandler{qmcKeyResolver: resolver}
	items := []service.BatchItem{
		{Path: paths[0], Name: filepath.Base(paths[0])},
		{Path: paths[1], Name: filepath.Base(paths[1])},
	}

	keys := handler.resolveQMCBatchKeys(context.Background(), items)
	if len(resolver.resources) != 1 {
		t.Fatalf("resolver resources = %d, want 1 deduplicated resource", len(resolver.resources))
	}
	for _, path := range paths {
		if keys[path].err != nil || keys[path].ekey == "" {
			t.Fatalf("key for %s = %#v", path, keys[path])
		}
	}
}

func TestResolveQMCBatchKeysPreservesResourceCase(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "upper.mgg"), filepath.Join(root, "lower.mgg")}
	fixtures := [][]byte{
		append([]byte("encrypted"), testMusicExFooter(t, "CaseMID", "CaseFile.mgg")...),
		append([]byte("encrypted"), testMusicExFooter(t, "casemid", "casefile.mgg")...),
	}
	for index, path := range paths {
		if err := os.WriteFile(path, fixtures[index], 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}
	resolver := &fakeQMCBatchResolver{}
	handler := &ConvertHandler{qmcKeyResolver: resolver}
	items := []service.BatchItem{{Path: paths[0], Name: filepath.Base(paths[0])}, {Path: paths[1], Name: filepath.Base(paths[1])}}

	handler.resolveQMCBatchKeys(context.Background(), items)
	if len(resolver.resources) != 2 {
		t.Fatalf("resolver resources = %#v, want two case-distinct resources", resolver.resources)
	}
}

func TestResolveQMCBatchKeysMapsCredentialFailureWithoutSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.mgg")
	fixture := append([]byte("encrypted"), testMusicExFooter(t, "001MID", "O8M000MID.mgg")...)
	if err := os.WriteFile(path, fixture, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	resolver := &fakeQMCBatchResolver{results: []qmckey.BatchResult{{Err: qmckey.ErrSessionExpired}}}
	handler := &ConvertHandler{qmcKeyResolver: resolver}

	keys := handler.resolveQMCBatchKeys(context.Background(), []service.BatchItem{{Path: path, Name: filepath.Base(path)}})
	var appErr *AppError
	if !errors.As(keys[path].err, &appErr) || appErr.Code != ErrQMCSessionExpired {
		t.Fatalf("resolved error = %#v, want %s", keys[path].err, ErrQMCSessionExpired)
	}
	for _, forbidden := range []string{"authst", "uin", "ekey"} {
		if strings.Contains(strings.ToLower(appErr.Detail), forbidden) {
			t.Fatalf("error detail exposes sensitive field name %q: %s", forbidden, appErr.Detail)
		}
	}
}

func TestMapQMCKeyDeadlineIsNetworkFailure(t *testing.T) {
	var appErr *AppError
	if err := mapQMCKeyError(context.DeadlineExceeded); !errors.As(err, &appErr) || appErr.Code != ErrQMCNetwork {
		t.Fatalf("mapQMCKeyError(deadline) = %#v, want %s", err, ErrQMCNetwork)
	}
}

func testMusicExFooter(t *testing.T, mediaMID, filename string) []byte {
	t.Helper()
	footer := make([]byte, 0xC0)
	binary.LittleEndian.PutUint32(footer[0:4], 42)
	binary.LittleEndian.PutUint32(footer[4:8], 2)
	binary.LittleEndian.PutUint32(footer[8:12], 5)
	writeHandlerUTF16(t, footer[0x0C:0x48], mediaMID)
	writeHandlerUTF16(t, footer[0x48:0x8C], filename)
	binary.LittleEndian.PutUint32(footer[0xB0:0xB4], uint32(len(footer)))
	binary.LittleEndian.PutUint32(footer[0xB4:0xB8], 1)
	copy(footer[0xB8:], []byte("musicex\x00"))
	return footer
}

func writeHandlerUTF16(t *testing.T, dst []byte, value string) {
	t.Helper()
	encoded := utf16.Encode([]rune(value))
	if (len(encoded)+1)*2 > len(dst) {
		t.Fatalf("UTF-16 fixture %q does not fit", value)
	}
	for index, unit := range encoded {
		binary.LittleEndian.PutUint16(dst[index*2:index*2+2], unit)
	}
}
