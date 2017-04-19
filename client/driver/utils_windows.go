package driver

import (
	"os/exec"
)

// TODO Figure out if this is needed in Wondows
func isolateCommand(cmd *exec.Cmd) {
}

// setChroot is a noop on Windows
func setChroot(cmd *exec.Cmd, chroot string) {
}
