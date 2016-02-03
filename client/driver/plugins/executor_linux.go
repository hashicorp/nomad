package plugins

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"
)

type LinuxExecutor struct {
	ctx *ExecutorContext

	logger *log.Logger
}

func NewExecutor(logger *log.Logger) Executor {
	return &LinuxExecutor{logger: logger}
}

func (e *LinuxExecutor) LaunchCmd(command *ExecCommand, ctx *ExecutorContext) (*ProcessState, error) {
	var cmd exec.Cmd
	cmd.Path = command.Cmd
	cmd.Args = append([]string{name}, args...)
	if filepath.Base(command.Cmd) == command.Cmd {
		if lp, err := exec.LookPath(command.Cmd); err != nil {
		} else {
			cmd.Path = lp
		}
	}
	cmd.Env = ctx.TaskEnv.EnvList()
	return &ProcessState{Pid: 5, ExitCode: -1, Time: time.Now()}, nil
}

func (e *LinuxExecutor) Wait() (*ProcessState, error) {
	time.Sleep(5 * time.Second)
	return &ProcessState{Pid: 0, ExitCode: 0, Time: time.Now()}, nil
}

func (e *LinuxExecutor) Exit() error {
	return nil
}

func (e *LinuxExecutor) ShutDown() (*ProcessState, error) {
	return nil
}
