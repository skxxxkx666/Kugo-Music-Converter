//go:build windows

package utils

import (
	"os/exec"
	"testing"
)

func TestConfigureBackgroundCommandHidesConsoleWindow(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "exit", "0")
	ConfigureBackgroundCommand(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("HideWindow is false")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("CreationFlags = %#x, want CREATE_NO_WINDOW", cmd.SysProcAttr.CreationFlags)
	}
}
