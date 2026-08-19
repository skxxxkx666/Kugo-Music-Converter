//go:build webview2bundle

package webview2bundle

import (
	_ "embed"
	"strings"
)

//go:embed payload/webview2-runtime.cab
var runtimeCAB []byte

//go:embed payload/webview2-runtime.version
var runtimeVersion string

//go:embed payload/webview2-runtime.sha256
var runtimeSHA256 string

func EmbeddedPayload() PayloadInfo {
	return PayloadInfo{
		CAB:            runtimeCAB,
		Version:        strings.TrimSpace(runtimeVersion),
		ExpectedSHA256: strings.TrimSpace(runtimeSHA256),
	}
}
