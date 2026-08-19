//go:build !webview2bundle

package webview2bundle

func EmbeddedPayload() PayloadInfo {
	return PayloadInfo{}
}
