//go:build windows

package webview2bundle

import (
	"fmt"
	"os/exec"
	"strings"
)

func configureRuntimeAccess(runtimeDir string) error {
	command := exec.Command(
		"icacls.exe",
		runtimeDir,
		"/grant",
		"*S-1-15-2-2:(OI)(CI)(RX)",
		"*S-1-15-2-1:(OI)(CI)(RX)",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("grant webview2 AppContainer access: %s", message)
	}
	return nil
}
