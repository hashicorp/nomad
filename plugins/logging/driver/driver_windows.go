//go:build windows
// +build windows

package driver

import (
	"os/exec"
)

func isolateCommand(cmd *exec.Cmd) {}
