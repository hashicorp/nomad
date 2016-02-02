package plugins

import (
	"log"
	"os/exec"
	"time"
)

type LinuxExecutor struct {
	cmd *exec.Cmd
	ctx *ExecutorContext

	log *log.Logger
}

func NewExecutor() Executor {
	return &LinuxExecutor{}
}

func (e *LinuxExecutor) LaunchCmd(cmd *exec.Cmd, ctx *ExecutorContext) (*ProcessState, error) {
	return &ProcessState{Pid: 5, ExitCode: -1, Time: time.Now()}, nil
}

func (e *LinuxExecutor) Wait() (*ProcessState, error) {
	time.Sleep(5 * time.Second)
	return &ProcessState{Pid: 0, ExitCode: 0, Time: time.Now()}, nil
}

func (e *LinuxExecutor) Exit() (*ProcessState, error) {
	return &ProcessState{Pid: 0, ExitCode: 0, Time: time.Now()}, nil
}

func (e *LinuxExecutor) ShutDown() (*ProcessState, error) {
	return &ProcessState{Pid: 0, ExitCode: 0, Time: time.Now()}, nil
}
