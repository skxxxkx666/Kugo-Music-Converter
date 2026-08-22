package qmckey

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseUIN(t *testing.T) {
	for _, fixture := range [][]byte{
		[]byte("[Account]\r\nUin=123456\r\n"),
		[]byte("# comment\nuIN = 123456\n"),
		append([]byte{0xef, 0xbb, 0xbf}, []byte("UIN=123456")...),
		{0xff, 0xfe, 'U', 0, 'i', 0, 'n', 0, '=', 0, '1', 0, '2', 0, '3', 0, '4', 0, '5', 0, '6', 0},
	} {
		if got := parseUIN(fixture); got != "123456" {
			t.Fatalf("parseUIN() = %q", got)
		}
	}
	for _, fixture := range [][]byte{[]byte("Uin=0"), []byte("Uin=user"), []byte("Other=123")} {
		if got := parseUIN(fixture); got != "" {
			t.Fatalf("parseUIN() = %q, want empty", got)
		}
	}
}

func TestAuthTokenCandidates(t *testing.T) {
	const token = "AABBCCDDEEFF00112233445566778899=="
	data := []byte(`prefix {"authst": "` + token + `"} suffix`)
	got := authTokenCandidates(data)
	if len(got) != 1 || got[0] != token {
		t.Fatalf("authTokenCandidates() = %#v", got)
	}

	if got := authTokenCandidates([]byte("binary\x00" + token + "\x00end")); len(got) != 0 {
		t.Fatalf("unlabelled Base64 candidates = %#v, want none", got)
	}

	if got := authTokenCandidates([]byte(`{"authst":"short"}`)); len(got) != 0 {
		t.Fatalf("short candidates = %#v", got)
	}
}

type countingSource struct {
	loads int
}

func (s *countingSource) Load(context.Context) (credentials, error) {
	s.loads++
	return credentials{}, ErrNotLoggedIn
}

func TestResolverSkipsCredentialsWhenAllResourcesInvalid(t *testing.T) {
	source := &countingSource{}
	resolver := &service{source: source, fetcher: keyFetcherFunc(func(context.Context, string, string, Resource) (string, error) {
		t.Fatal("fetcher must not be called")
		return "", nil
	})}
	results := resolver.ResolveBatch(context.Background(), []Resource{{MediaMid: "invalid value", Filename: "song.exe"}})
	if source.loads != 0 {
		t.Fatalf("credential loads = %d, want 0", source.loads)
	}
	if len(results) != 1 || !errors.Is(results[0].Err, ErrProtocol) {
		t.Fatalf("results = %#v", results)
	}
}

type fakeSource struct {
	credentials credentials
	err         error
}

func (s fakeSource) Load(context.Context) (credentials, error) {
	return s.credentials, s.err
}

type fakeFetcher struct {
	attempts []string
}

func (f *fakeFetcher) Fetch(_ context.Context, _, token string, _ Resource) (string, error) {
	f.attempts = append(f.attempts, token)
	if len(f.attempts) == 1 {
		return "", ErrSessionExpired
	}
	return strings.Repeat("e", 32), nil
}

func TestResolverTriesSessionCandidates(t *testing.T) {
	fetcher := &fakeFetcher{}
	resolver := &service{
		source: fakeSource{credentials: credentials{
			uin:        "123456",
			authTokens: []string{"AABBCCDDEEFF0011", "AABBCCDDEEFF0022"},
		}},
		fetcher: fetcher,
	}
	ekey, err := resolver.Resolve(context.Background(), Resource{MediaMid: "001ABC", Filename: "song.mgg"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ekey == "" || len(fetcher.attempts) != 2 {
		t.Fatalf("ekey=%q attempts=%#v", ekey, fetcher.attempts)
	}
}

func TestResolverStopsOnEntitlementFailure(t *testing.T) {
	resolver := &service{
		source: fakeSource{credentials: credentials{uin: "123456", authTokens: []string{"AABBCCDDEEFF0011"}}},
		fetcher: keyFetcherFunc(func(context.Context, string, string, Resource) (string, error) {
			return "", ErrEntitlement
		}),
	}
	_, err := resolver.Resolve(context.Background(), Resource{MediaMid: "001ABC", Filename: "song.mgg"})
	if !errors.Is(err, ErrEntitlement) {
		t.Fatalf("Resolve() error = %v", err)
	}
}

type keyFetcherFunc func(context.Context, string, string, Resource) (string, error)

func (f keyFetcherFunc) Fetch(ctx context.Context, uin, token string, resource Resource) (string, error) {
	return f(ctx, uin, token, resource)
}
