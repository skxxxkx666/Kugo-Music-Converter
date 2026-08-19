//go:build !windows

package webview2bundle

func configureRuntimeAccess(string) error {
	return nil
}
