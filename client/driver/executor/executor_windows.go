package executor

import (
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

func (e *UniversalExecutor) LaunchSyslogServer(ctx *ExecutorContext) (*SyslogServerState, error) {
	return nil, nil
}

func (e *UniversalExecutor) wait() {
	defer close(e.processExited)
	err := e.cmd.Wait()
	ic := &cstructs.IsolationConfig{Cgroup: e.groups, CgroupPaths: e.cgPaths}
	if err == nil {
		e.exitState = &ProcessState{Pid: 0, ExitCode: 0, IsolationConfig: ic, Time: time.Now()}
		return
	}
	exitCode := 1
	var signal int
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(windows.WaitStatus); ok {
			exitCode = status.ExitStatus()
			if status.Signaled() {
				signal = int(status.Signal())
				exitCode = 128 + signal
			}
		}
	} else {
		e.logger.Printf("[DEBUG] executor: unexpected Wait() error type: %v", err)
	}

	e.exitState = &ProcessState{Pid: 0, ExitCode: exitCode, Signal: signal, IsolationConfig: ic, Time: time.Now()}
}
