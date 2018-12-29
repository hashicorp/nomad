package utils

import (
	"os/exec"
)

// TODO Figure out if this is needed in Windows
func isolateCommand(cmd *exec.Cmd) {}

// IsUnixRoot returns true if system is a unix system and the effective uid of user is root
func IsUnixRoot() bool {
	return false
}
