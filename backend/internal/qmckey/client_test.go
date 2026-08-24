package qmckey

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestEVKeyClientRequestAndResponse(t *testing.T) {
	const (
		uin   = "123456"
		token = "AABBCCDDEEFF00112233445566778899=="
		ekey  = "0123456789abcdef0123456789abcdef"
	)
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.String() != evKeyEndpoint {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
		if request.Header.Get("Content-Type") != "application/json" || request.Header.Get("Referer") != "https://y.qq.com/" {
			t.Fatalf("unexpected headers: %#v", request.Header)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		comm := payload["comm"].(map[string]any)
		if comm["authst"] != token || comm["uin"] != uin || comm["ct"] != "19" || comm["cv"] != "1859" || comm["tmeLoginType"] != "3" {
			t.Fatalf("unexpected comm: %#v", comm)
		}
		req := payload["req_1"].(map[string]any)
		if req["module"] != "music.vkey.GetEVkey" || req["method"] != "CgiGetEVkey" {
			t.Fatalf("unexpected request envelope: %#v", req)
		}
		param := req["param"].(map[string]any)
		if param["guid"] != "10000" || param["platform"] != "27" || param["uin"] != uin {
			t.Fatalf("unexpected params: %#v", param)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"code":0,"req_1":{"code":0,"data":{"midurlinfo":[{"result":0,"ekey":"` + ekey + `"}]}}}`,
			)),
			Request: request,
		}, nil
	})
	client := newEVKeyClient(&http.Client{Transport: transport})
	got, err := client.Fetch(context.Background(), uin, token, Resource{
		SongID:   42,
		MediaMid: "001ABCdef_123",
		Filename: "F0M000001ABCdef.mflac",
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got != ekey {
		t.Fatalf("Fetch() = %q, want ekey", got)
	}
}

func TestEVKeyClientClassifiesErrorsWithoutCredentials(t *testing.T) {
	const (
		uin   = "123456"
		token = "AABBCCDDEEFF00112233445566778899=="
	)
	tests := []struct {
		name string
		body string
		want error
	}{
		{name: "expired", body: `{"req_1":{"code":104005}}`, want: ErrSessionExpired},
		{name: "entitlement", body: `{"req_1":{"code":104003}}`, want: ErrEntitlement},
		{name: "missing-code", body: `{"req_1":{}}`, want: ErrProtocol},
		{name: "empty-key", body: `{"req_1":{"code":0,"data":{"midurlinfo":[{"result":0,"ekey":""}]}}}`, want: ErrProtocol},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newEVKeyClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header), Request: request}, nil
			})})
			_, err := client.Fetch(context.Background(), uin, token, Resource{MediaMid: "001ABC", Filename: "O8M000001ABC.mgg"})
			if !errors.Is(err, test.want) {
				t.Fatalf("Fetch() error = %v, want %v", err, test.want)
			}
			for _, secret := range []string{uin, token} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error leaks credential %q: %v", secret, err)
				}
			}
		})
	}
}

func TestEVKeyClientRejectsOversizedResponse(t *testing.T) {
	client := newEVKeyClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", maxEVKeyResponse+1))),
			Request:    request,
		}, nil
	})})
	_, err := client.Fetch(context.Background(), "123456", "AABBCCDDEEFF00112233", Resource{MediaMid: "001ABC", Filename: "song.mgg"})
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("Fetch() error = %v, want ErrProtocol", err)
	}
}

func TestEVKeyClientHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := newEVKeyClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, request.Context().Err()
	})})
	_, err := client.Fetch(ctx, "123456", "AABBCCDDEEFF00112233", Resource{MediaMid: "001ABC", Filename: "song.mgg"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch() error = %v, want context.Canceled", err)
	}
}

func TestEVKeyClientRejectsRedirectResponse(t *testing.T) {
	client := newEVKeyClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://example.invalid/"}}, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
	})})
	_, err := client.Fetch(context.Background(), "123456", "AABBCCDDEEFF00112233", Resource{MediaMid: "001ABC", Filename: "song.mgg"})
	if !errors.Is(err, ErrNetwork) {
		t.Fatalf("Fetch() error = %v, want ErrNetwork", err)
	}
}

func TestValidateResource(t *testing.T) {
	for _, resource := range []Resource{
		{},
		{MediaMid: "has space", Filename: "song.mgg"},
		{MediaMid: "001ABC", Filename: `..\\song.mgg`},
		{MediaMid: " 001ABC", Filename: "song.mgg"},
		{MediaMid: "001ABC", Filename: "song.mgg "},
		{MediaMid: "001ABC", Filename: "song.exe"},
	} {
		if err := validateResource(resource); !errors.Is(err, ErrProtocol) {
			t.Fatalf("validateResource(%#v) = %v, want ErrProtocol", resource, err)
		}
	}
	if err := validateResource(Resource{MediaMid: "001ABC_-", Filename: "song.mflac"}); err != nil {
		t.Fatalf("validateResource() error = %v", err)
	}
}
