// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package executor

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// configure new process group for child process and creates a JobObject for the
// executor. Children of the executor will be created in the same JobObject
// Ref: https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects
func (e *UniversalExecutor) setNewProcessGroup() error {
	// We need to check that as build flags includes windows for this file
	if e.childCmd.SysProcAttr == nil {
		e.childCmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	e.childCmd.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP

	// note: we don't call CloseHandle on this job handle because we need to
	// hold onto it until the executor exits
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("could not create Windows job object for executor: %w", err)
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)))
	if err != nil {
		return fmt.Errorf("could not configure Windows job object for executor: %w", err)
	}

	handle := windows.CurrentProcess()
	err = windows.AssignProcessToJobObject(job, handle)
	if err != nil {
		return fmt.Errorf("could not assign executor to Windows job object: %w", err)
	}

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
