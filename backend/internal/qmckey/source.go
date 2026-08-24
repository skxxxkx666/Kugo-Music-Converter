package qmckey

import (
	"bytes"
	"encoding/binary"
	"regexp"
	"strings"
	"unicode/utf16"
)

const (
	maxCredentialFileSize  = 1 << 20
	maxAuthTokenLength     = 4096
	maxAuthTokenCandidates = 8
)

var authJSONPattern = regexp.MustCompile(`(?i)"authst"\s*:\s*"([A-Za-z0-9+/_=-]+)"`)

func parseUIN(data []byte) string {
	text := decodeConfigText(data)
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "uin") {
			continue
		}
		candidate := strings.TrimSpace(value)
		if candidate != "0" && uinPattern.MatchString(candidate) {
			return candidate
		}
	}
	return ""
}

func decodeConfigText(data []byte) string {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if len(data) >= 2 && data[0] == 0xff && data[1] == 0xfe {
		words := make([]uint16, 0, (len(data)-2)/2)
		for i := 2; i+1 < len(data); i += 2 {
			words = append(words, binary.LittleEndian.Uint16(data[i:i+2]))
		}
		return string(utf16.Decode(words))
	}
	return string(data)
}

func authTokenCandidates(data []byte) []string {
	return jsonAuthTokenCandidates(data)
}

func jsonAuthTokenCandidates(data []byte) []string {
	matches := authJSONPattern.FindAllSubmatch(data, maxAuthTokenCandidates)
	candidates := make([]string, 0, maxAuthTokenCandidates)
	seen := make(map[string]struct{}, maxAuthTokenCandidates)
	for _, match := range matches {
		if len(match) == 2 {
			appendAuthCandidate(&candidates, seen, string(match[1]))
		}
	}
	return candidates
}

func appendAuthCandidate(dst *[]string, seen map[string]struct{}, candidate string) {
	if len(*dst) >= maxAuthTokenCandidates {
		return
	}
	candidate = strings.TrimSpace(candidate)
	if !validAuthToken(candidate) {
		return
	}
	if _, ok := seen[candidate]; ok {
		return
	}
	seen[candidate] = struct{}{}
	*dst = append(*dst, candidate)
}

func validAuthToken(candidate string) bool {
	if len(candidate) < 16 || len(candidate) > maxAuthTokenLength {
		return false
	}
	for _, ch := range candidate {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '+', ch == '/', ch == '-', ch == '_', ch == '=':
		default:
			return false
		}
	}
	return true
}
