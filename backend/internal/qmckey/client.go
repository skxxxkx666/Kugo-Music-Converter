package qmckey

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

const (
	evKeyEndpoint       = "https://u.y.qq.com/cgi-bin/musicu.fcg"
	maxEVKeyResponse    = 1 << 20
	maxResourceNameSize = 255
)

var (
	mediaMidPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,60}$`)
	uinPattern      = regexp.MustCompile(`^[0-9]{1,20}$`)
)

type evKeyClient struct {
	httpClient *http.Client
	endpoint   string
}

func newEVKeyClient(client *http.Client) *evKeyClient {
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.ResponseHeaderTimeout = 10 * time.Second
		transport.TLSHandshakeTimeout = 10 * time.Second
		client = &http.Client{
			Transport: transport,
			Timeout:   20 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &evKeyClient{httpClient: client, endpoint: evKeyEndpoint}
}

func validateResource(resource Resource) error {
	mediaMID := strings.TrimSpace(resource.MediaMid)
	if mediaMID != resource.MediaMid || !mediaMidPattern.MatchString(mediaMID) {
		return fmt.Errorf("%w: invalid media identifier", ErrProtocol)
	}
	name := strings.TrimSpace(resource.Filename)
	if name != resource.Filename || name == "" || len(name) > maxResourceNameSize || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return fmt.Errorf("%w: invalid resource filename", ErrProtocol)
	}
	for _, ch := range name {
		if ch < 0x20 || ch == 0x7f {
			return fmt.Errorf("%w: invalid resource filename", ErrProtocol)
		}
	}
	ext := strings.ToLower(path.Ext(name))
	if ext != ".mflac" && ext != ".mgg" {
		return fmt.Errorf("%w: unsupported resource filename", ErrProtocol)
	}
	return nil
}

func (c *evKeyClient) Fetch(ctx context.Context, uin, authToken string, resource Resource) (string, error) {
	if !uinPattern.MatchString(strings.TrimSpace(uin)) || !validAuthToken(authToken) {
		return "", ErrNotLoggedIn
	}
	if err := validateResource(resource); err != nil {
		return "", err
	}

	payload := map[string]any{
		"comm": map[string]any{
			"authst":       authToken,
			"ct":           "19",
			"cv":           "1859",
			"uin":          uin,
			"tmeLoginType": "3",
		},
		"req_1": map[string]any{
			"module": "music.vkey.GetEVkey",
			"method": "CgiGetEVkey",
			"param": map[string]any{
				"filename":  []string{resource.Filename},
				"guid":      "10000",
				"songmid":   []string{resource.MediaMid},
				"songtype":  []int{1},
				"uin":       uin,
				"loginflag": 1,
				"platform":  "27",
				"ctx":       1,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("%w: encode request", ErrProtocol)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%w: create request", ErrProtocol)
	}
	if parsed, parseErr := url.Parse(c.endpoint); parseErr != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "u.y.qq.com") {
		return "", fmt.Errorf("%w: untrusted endpoint", ErrProtocol)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Referer", "https://y.qq.com/")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")

	response, err := c.httpClient.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", ErrNetwork
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", ErrNetwork
	}

	limited := io.LimitReader(response.Body, maxEVKeyResponse+1)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		return "", ErrNetwork
	}
	if len(responseBody) > maxEVKeyResponse {
		return "", fmt.Errorf("%w: response too large", ErrProtocol)
	}

	var decoded struct {
		Request struct {
			Code *int `json:"code"`
			Data *struct {
				Items []struct {
					Result *int   `json:"result"`
					EKey   string `json:"ekey"`
				} `json:"midurlinfo"`
			} `json:"data"`
		} `json:"req_1"`
	}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", fmt.Errorf("%w: invalid JSON", ErrProtocol)
	}
	if decoded.Request.Code == nil || *decoded.Request.Code != 0 || decoded.Request.Data == nil || len(decoded.Request.Data.Items) == 0 {
		return "", classifyEVKeyCode(decoded.Request.Code)
	}
	item := decoded.Request.Data.Items[0]
	if item.Result == nil || *item.Result != 0 {
		return "", classifyEVKeyCode(item.Result)
	}
	ekey := strings.TrimSpace(item.EKey)
	if len(ekey) < 16 || len(ekey) > 8192 {
		return "", fmt.Errorf("%w: empty or implausible key", ErrProtocol)
	}
	return ekey, nil
}

func classifyEVKeyCode(code *int) error {
	if code == nil {
		return fmt.Errorf("%w: missing result code", ErrProtocol)
	}
	switch *code {
	case 104003, 104004, 104013:
		return ErrEntitlement
	case 104005:
		return ErrSessionExpired
	default:
		return fmt.Errorf("%w: server rejected request (%d)", ErrProtocol, *code)
	}
}
