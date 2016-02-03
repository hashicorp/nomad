// +build !linux

package plugins

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
)

type BasicExecutor struct {
	logger *log.Logger
	cmd    exec.Cmd

	taskDir string
}

func NewExecutor(logger *log.Logger) Executor {
	return &BasicExecutor{logger: logger}
}

func (e *BasicExecutor) LaunchCmd(command *ExecCommand, ctx *ExecutorContext) (*ProcessState, error) {
	e.cmd.Path = command.Cmd
	e.cmd.Args = append([]string{command.Cmd}, command.Args...)
	e.cmd.Path = ctx.TaskEnv.ReplaceEnv(e.cmd.Path)
	e.cmd.Args = ctx.TaskEnv.ParseAndReplace(e.cmd.Args)

	if filepath.Base(command.Cmd) == command.Cmd {
		if lp, err := exec.LookPath(command.Cmd); err != nil {
		} else {
			e.cmd.Path = lp
		}
	}
	e.configureTaskDir(ctx.Task.Name, ctx.AllocDir)
	e.cmd.Env = ctx.TaskEnv.EnvList()
	stdoPath := filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stdout", ctx.Task.Name))
	stdo, err := os.OpenFile(stdoPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	e.cmd.Stdout = stdo

	stdePath := filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stderr", ctx.Task.Name))
	stde, err := os.OpenFile(stdePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	e.cmd.Stderr = stde
	if err := e.cmd.Start(); err != nil {
		return nil, err
	}

	return &ProcessState{Pid: 5, ExitCode: -1, Time: time.Now()}, nil
}

func (e *BasicExecutor) Wait() (*ProcessState, error) {
	err := e.cmd.Wait()
	if err == nil {
		return &ProcessState{Pid: 0, ExitCode: 0, Time: time.Now()}, nil
	}
	exitCode := 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
		}
	}
	return &ProcessState{Pid: 0, ExitCode: exitCode, Time: time.Now()}, nil
}

func (e *BasicExecutor) Exit() error {
	proc, err := os.FindProcess(e.cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failied to find user process %v: %v", e.cmd.Process.Pid, err)
	}
	return proc.Kill()
}

func (e *BasicExecutor) ShutDown() error {
	proc, err := os.FindProcess(e.cmd.Process.Pid)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return proc.Kill()
	}
	return proc.Signal(os.Interrupt)
}

func (e *BasicExecutor) configureTaskDir(taskName string, allocDir *allocdir.AllocDir) error {
	taskDir, ok := allocDir.TaskDirs[taskName]
	e.taskDir = taskDir
	if !ok {
		return fmt.Errorf("Couldn't find task directory for task %v", taskName)
	}
	e.cmd.Dir = taskDir
	return nil
}
