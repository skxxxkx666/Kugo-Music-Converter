//go:build windows

package main

import "syscall"

// init sets the Windows console codepage to UTF-8 (65001) so that
// Chinese log messages display correctly regardless of the system locale.
func init() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")

	if proc := kernel32.NewProc("SetConsoleOutputCP"); proc != nil {
		proc.Call(65001) //nolint:errcheck
	}
	if proc := kernel32.NewProc("SetConsoleCP"); proc != nil {
		proc.Call(65001) //nolint:errcheck
	}
}
