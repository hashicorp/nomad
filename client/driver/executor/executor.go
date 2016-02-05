package executor

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ExecutorContext is a wrapper to hold context to configure the command user
// wants to run
type ExecutorContext struct {
	TaskEnv          *env.TaskEnvironment
	AllocDir         *allocdir.AllocDir
	TaskName         string
	TaskResources    *structs.Resources
	FSIsolation      bool
	ResourceLimits   bool
	UnprivilegedUser bool
}

// ExecCommand is a wrapper to hold the user command
type ExecCommand struct {
	Cmd  string
	Args []string
}

// ProcessState holds information about the state of
// a user process
type ProcessState struct {
	Pid      int
	ExitCode int
	Time     time.Time
}

// Executor is the interface which allows a driver to launch and supervise
// a process user wants to run
type Executor interface {
	LaunchCmd(command *ExecCommand, ctx *ExecutorContext) (*ProcessState, error)
	Wait() (*ProcessState, error)
	ShutDown() error
	Exit() error
}

// UniversalExecutor is an implementation of the Executor which launches and
// supervises processes. In addition to process supervision it provides resource
// and file system isolation
type UniversalExecutor struct {
	cmd exec.Cmd
	ctx *ExecutorContext

	taskDir       string
	groups        *cgroupConfig.Cgroup
	exitState     *ProcessState
	processExited chan interface{}

	logger *log.Logger
	lock   sync.Mutex
}

// NewExecutor returns an Executor
func NewExecutor(logger *log.Logger) Executor {
	return &UniversalExecutor{logger: logger, processExited: make(chan interface{})}
}

// LaunchCmd launches a process and returns it's state. It also configures an
// applies isolation on certain platforms.
func (e *UniversalExecutor) LaunchCmd(command *ExecCommand, ctx *ExecutorContext) (*ProcessState, error) {
	e.ctx = ctx

	if err := e.configureTaskDir(); err != nil {
		return nil, err
	}
	if err := e.configureIsolation(); err != nil {
		return nil, err
	}

	if e.ctx.UnprivilegedUser {
		if err := e.runAs("nobody"); err != nil {
			return nil, err
		}
	}

	stdoPath := filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stdout", ctx.TaskName))
	stdo, err := os.OpenFile(stdoPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	e.cmd.Stdout = stdo

	stdePath := filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stderr", ctx.TaskName))
	stde, err := os.OpenFile(stdePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	e.cmd.Stderr = stde

	e.cmd.Env = ctx.TaskEnv.EnvList()

	e.cmd.Path = ctx.TaskEnv.ReplaceEnv(command.Cmd)
	e.cmd.Args = append([]string{e.cmd.Path}, ctx.TaskEnv.ParseAndReplace(command.Args)...)
	if filepath.Base(command.Cmd) == command.Cmd {
		if lp, err := exec.LookPath(command.Cmd); err != nil {
		} else {
			e.cmd.Path = lp
		}
	}

	if err := e.cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting command: %v", err)
	}

	e.applyLimits()
	go e.wait()
	return &ProcessState{Pid: e.cmd.Process.Pid, ExitCode: -1, Time: time.Now()}, nil
}

// Wait waits until a process has exited and returns it's exitcode and errors
func (e *UniversalExecutor) Wait() (*ProcessState, error) {
	<-e.processExited
	return e.exitState, nil
}

func (e *UniversalExecutor) wait() {
	defer close(e.processExited)
	err := e.cmd.Wait()
	if err == nil {
		e.exitState = &ProcessState{Pid: 0, ExitCode: 0, Time: time.Now()}
		return
	}
	exitCode := 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
		}
	}
	if e.ctx.FSIsolation {
		e.removeChrootMounts()
	}
	if e.ctx.ResourceLimits {
		e.destroyCgroup()
	}
	e.exitState = &ProcessState{Pid: 0, ExitCode: exitCode, Time: time.Now()}
}

// Exit cleans up the alloc directory, destroys cgroups and kills the user
// process
func (e *UniversalExecutor) Exit() error {
	e.logger.Printf("[INFO] Exiting plugin for task %q", e.ctx.TaskName)
	if e.cmd.Process == nil {
		return fmt.Errorf("executor.exit error: no process found")
	}
	proc, err := os.FindProcess(e.cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failied to find user process %v: %v", e.cmd.Process.Pid, err)
	}
	if e.ctx.FSIsolation {
		e.removeChrootMounts()
	}
	if e.ctx.ResourceLimits {
		e.destroyCgroup()
	}
	if err = proc.Kill(); err != nil {
		e.logger.Printf("[DEBUG] executor.exit error: %v", err)
	}
	return nil
}

// Shutdown sends an interrupt signal to the user process
func (e *UniversalExecutor) ShutDown() error {
	if e.cmd.Process == nil {
		return fmt.Errorf("executor.shutdown error: no process found")
	}
	proc, err := os.FindProcess(e.cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("executor.shutdown error: %v", err)
	}
	if runtime.GOOS == "windows" {
		return proc.Kill()
	}
	if err = proc.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("executor.shutdown error: %v", err)
	}
	return nil
}

func (e *UniversalExecutor) configureTaskDir() error {
	taskDir, ok := e.ctx.AllocDir.TaskDirs[e.ctx.TaskName]
	e.taskDir = taskDir
	if !ok {
		return fmt.Errorf("Couldn't find task directory for task %v", e.ctx.TaskName)
	}
	e.cmd.Dir = taskDir
	return nil
}
