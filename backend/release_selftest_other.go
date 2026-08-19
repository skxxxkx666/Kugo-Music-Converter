//go:build !windows

package main

import "os/exec"

func configureReleaseSelfTestCommand(_ *exec.Cmd) {}
