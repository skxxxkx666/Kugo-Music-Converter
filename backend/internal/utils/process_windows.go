//go:build windows

package utils

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// ConfigureBackgroundCommand prevents console applications such as FFmpeg
// from creating a visible terminal window when launched by the desktop app.
func ConfigureBackgroundCommand(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
