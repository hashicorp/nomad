// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package executor

import (
	"os"
	"syscall"
)

// configure new process group for child process
func (e *UniversalExecutor) setNewProcessGroup() error {
	if e.cmd.SysProcAttr == nil {
		e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	e.cmd.SysProcAttr.Setpgid = true
	return nil
}

// Cleanup any still hanging user processes
func (e *UniversalExecutor) cleanupChildProcesses(proc *os.Process) error {
	// If new process group was created upon command execution
	// we can kill the whole process group now to cleanup any leftovers.
	if e.cmd.SysProcAttr != nil && e.cmd.SysProcAttr.Setpgid {
		if err := syscall.Kill(-proc.Pid, syscall.SIGKILL); err != nil && err.Error() != noSuchProcessErr {
			return err
		}
		return nil
	}
	return proc.Kill()
}
