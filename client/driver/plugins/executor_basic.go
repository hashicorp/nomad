// +build !linux

package plugins

import (
	"os/exec"
	"time"
)

type BasicExecutor struct {
}

func NewExecutor() Executor {
	return &BasicExecutor{}
}

func (e *BasicExecutor) LaunchCmd(cmd *exec.Cmd, ctx *ExecutorContext) (*ProcessState, error) {
	return &ProcessState{Pid: 5, ExitCode: -1, Time: time.Now()}, nil
}

func (e *BasicExecutor) Wait() (*ProcessState, error) {
	time.Sleep(5 * time.Second)
	return &ProcessState{Pid: 0, ExitCode: 0, Time: time.Now()}, nil
}

func (e *BasicExecutor) Exit() (*ProcessState, error) {
	return &ProcessState{Pid: 0, ExitCode: 0, Time: time.Now()}, nil
}

func (e *BasicExecutor) ShutDown() (*ProcessState, error) {
	return &ProcessState{Pid: 0, ExitCode: 0, Time: time.Now()}, nil
}
