// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package executor

import (
	"os"
	"os/exec"
	"syscall"
)

// isolateCommand sets the setsid flag in exec.Cmd to true so that the process
// becomes the process leader in a new session and doesn't receive signals that
// are sent to the parent process.
func isolateCommand(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}

func isProcessRunning(process *os.Process) bool {
	err := process.Signal(syscall.Signal(0))
	return err == nil
}
