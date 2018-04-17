// +build windows

package executor

import (
	"fmt"
	"io"
	"os"
	"syscall"

	syslog "github.com/RackSec/srslog"

	"github.com/hashicorp/nomad/client/driver/logging"
)

func (e *UniversalExecutor) LaunchSyslogServer() (*SyslogServerState, error) {
	// Ensure the context has been set first
	if e.ctx == nil {
		return nil, fmt.Errorf("SetContext must be called before launching the Syslog Server")
	}

	e.syslogChan = make(chan *logging.SyslogMessage, 2048)
	l, err := e.getListener(e.ctx.PortLowerBound, e.ctx.PortUpperBound)
	if err != nil {
		return nil, err
	}
	e.logger.Printf("[DEBUG] syslog-server: launching syslog server on addr: %v", l.Addr().String())
	if err := e.configureLoggers(); err != nil {
		return nil, err
	}

	e.syslogServer = logging.NewSyslogServer(l, e.syslogChan, e.logger)
	go e.syslogServer.Start()
	go e.collectLogs(e.lre, e.lro)
	syslogAddr := fmt.Sprintf("%s://%s", l.Addr().Network(), l.Addr().String())
	return &SyslogServerState{Addr: syslogAddr}, nil
}

func (e *UniversalExecutor) collectLogs(we io.Writer, wo io.Writer) {
	for logParts := range e.syslogChan {
		// If the severity of the log line is err then we write to stderr
		// otherwise all messages go to stdout
		if logParts.Severity == syslog.LOG_ERR {
			e.lre.Write(logParts.Message)
			e.lre.Write([]byte{'\n'})
		} else {
			e.lro.Write(logParts.Message)
			e.lro.Write([]byte{'\n'})
		}
	}
}

// configure new process group for child process
func (e *UniversalExecutor) setNewProcessGroup() error {
	// We need to check that as build flags includes windows for this file
	if e.cmd.SysProcAttr == nil {
		e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	e.cmd.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP
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

func (e *UniversalExecutor) shutdownProcess(proc *os.Process) error {
	if err := sendCtrlBreak(proc.Pid); err != nil {
		return fmt.Errorf("executor.shutdown error: %v", err)
	}
	e.logger.Printf("Sent Ctrl-Break to process %v", proc.Pid)

	return nil
}
