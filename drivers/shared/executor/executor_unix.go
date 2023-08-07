// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build unix

package executor

import (
	"fmt"
	"os"
	"syscall"
)

// configure new process group for child process
func (e *UniversalExecutor) setNewProcessGroup() error {
	if e.childCmd.SysProcAttr == nil {
		e.childCmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	e.childCmd.SysProcAttr.Setpgid = true
	return nil
}

// SIGKILL the process group starting at process.Pid
func (e *UniversalExecutor) killProcessTree(process *os.Process) error {
	pid := process.Pid
	negative := -pid // tells unix to kill entire process group
	signal := syscall.SIGKILL

	// If new process group was created upon command execution
	// we can kill the whole process group now to cleanup any leftovers.
	if e.childCmd.SysProcAttr != nil && e.childCmd.SysProcAttr.Setpgid {
		e.logger.Trace("sending sigkill to process group", "pid", pid, "negative", negative, "signal", signal)
		if err := syscall.Kill(negative, signal); err != nil && err.Error() != noSuchProcessErr {
			return err
		}
		return nil
	}
	return process.Kill()
}

// Only send the process a shutdown signal (default INT), doesn't
// necessarily kill it.
func (e *UniversalExecutor) shutdownProcess(sig os.Signal, proc *os.Process) error {
	if sig == nil {
		sig = os.Interrupt
	}

	if err := proc.Signal(sig); err != nil && err.Error() != finishedErr {
		return fmt.Errorf("executor shutdown error: %v", err)
	}

	return nil
}
