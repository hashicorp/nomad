package executor

import (
	"fmt"
	"os"
	"syscall"
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
func (e *UniversalExecutor) cleanupChildProcesses(proc *os.Process) error {
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
// like in https://golang.org/src/os/signal/signal_windows_test.go
func sendCtrlBreak(pid int) error {
	dll, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return fmt.Errorf("Error loading kernel32.dll: %v", err)
	}
	proc, err := dll.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return fmt.Errorf("Cannot find procedure GenerateConsoleCtrlEvent: %v", err)
	}
	result, _, err := proc.Call(syscall.CTRL_BREAK_EVENT, uintptr(pid))
	if result == 0 {
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
