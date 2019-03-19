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

// findLiveProcess looks up a given pid and attempts to validate its liveness.
func findLiveProcess(pid int) (*os.Process, error) {
	ps, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}

	// On Unix platforms FindProcess succeeds on dead processes
	err = ps.Signal(syscall.Signal(0))
	return ps, err
}
