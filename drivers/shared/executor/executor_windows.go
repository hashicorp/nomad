// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package executor

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// configure new process group for child process
func (e *UniversalExecutor) setNewProcessGroup() error {
	// We need to check that as build flags includes windows for this file
	if e.childCmd.SysProcAttr == nil {
		e.childCmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	e.childCmd.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP
	return nil
}

// Cleanup any still hanging user processes
func (e *UniversalExecutor) killProcessTree(proc *os.Process) error {
	// We must first verify if the process is still running.
	// (Windows process often lingered around after being reported as killed).
	handle, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE|syscall.SYNCHRONIZE|syscall.PROCESS_QUERY_INFORMATION, false, uint32(proc.Pid))
	if err != nil {
		return os.NewSyscallError("OpenProcess", err)
	}
	defer syscall.CloseHandle(handle)

	result, err := syscall.WaitForSingleObject(syscall.Handle(handle), 0)

	switch result {
	case syscall.WAIT_OBJECT_0:
		return nil
	case syscall.WAIT_TIMEOUT:
		// Process still running.  Just kill it.
		return proc.Kill()
	default:
		return os.NewSyscallError("WaitForSingleObject", err)
	}
}

// Send a Ctrl-Break signal for shutting down the process,
func sendCtrlBreak(pid int) error {
	err := windows.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, uint32(pid))
	if err != nil {
		return fmt.Errorf("Error sending ctrl-break event: %v", err)
	}
	return nil
}

// Send the process a Ctrl-Break event, allowing it to shutdown by itself
// before being Terminate.
func (e *UniversalExecutor) shutdownProcess(_ os.Signal, proc *os.Process) error {
	if err := sendCtrlBreak(proc.Pid); err != nil {
		return fmt.Errorf("executor shutdown error: %v", err)
	}
	e.logger.Debug("sent Ctrl-Break to process", "pid", proc.Pid)

	return nil
}
