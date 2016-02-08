package executor

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ExecutorContext holds context to configure the command user
// wants to run and isolate it
type ExecutorContext struct {

	// TaskEnv holds information about the environment of a Task
	TaskEnv *env.TaskEnvironment

	// AllocDir is the handle to do operations on the alloc dir of
	// the task
	AllocDir *allocdir.AllocDir

	// TaskName is the name of the Task
	TaskName string

	// TaskResources are the resource constraints for the Task
	TaskResources *structs.Resources

	// FSIsolation is a flag for drivers to impose file system
	// isolation on certain platforms
	FSIsolation bool

	// ResourceLimits is a flag for drivers to impose resource
	// contraints on a Task on certain platforms
	ResourceLimits bool

	// UnprivilegedUser is a flag for drivers to make the process
	// run as nobody
	UnprivilegedUser bool
}

// ExecCommand holds the user command and args. It's a lightweight replacement
// of exec.Cmd for serialization purposes.
type ExecCommand struct {
	Cmd  string
	Args []string
}

// ProcessState holds information about the state of a user process.
type ProcessState struct {
	Pid             int
	ExitCode        int
	Signal          int
	IsolationConfig cgroupConfig.Cgroup
	Time            time.Time
}

// Executor is the interface which allows a driver to launch and supervise
// a process
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
	e.logger.Printf("[DEBUG] executor: launching command %v %v", command.Cmd, strings.Join(command.Args, ""))

	e.ctx = ctx

	// configuring the task dir
	if err := e.configureTaskDir(); err != nil {
		return nil, err
	}

	// configuring the chroot, cgroup and enters the plugin process in the
	// chroot
	if err := e.configureIsolation(); err != nil {
		return nil, err
	}

	// setting the user of the process
	if e.ctx.UnprivilegedUser {
		if err := e.runAs("nobody"); err != nil {
			return nil, err
		}
	}

	// configuring log rotate
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

	// setting the env, path and args for the command
	e.ctx.TaskEnv.Build()
	e.cmd.Env = ctx.TaskEnv.EnvList()
	e.cmd.Path = ctx.TaskEnv.ReplaceEnv(command.Cmd)
	e.cmd.Args = append([]string{e.cmd.Path}, ctx.TaskEnv.ParseAndReplace(command.Args)...)
	if filepath.Base(command.Cmd) == command.Cmd {
		if lp, err := exec.LookPath(command.Cmd); err != nil {
		} else {
			e.cmd.Path = lp
		}
	}

	// starting the process
	if err := e.cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting command: %v", err)
	}

	go e.wait()
	return &ProcessState{Pid: e.cmd.Process.Pid, ExitCode: -1, IsolationConfig: *e.groups, Time: time.Now()}, nil
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
	var merr multierror.Error
	if e.cmd.Process != nil {
		proc, err := os.FindProcess(e.cmd.Process.Pid)
		if err != nil {
			e.logger.Printf("[ERROR] can't find process with pid: %v, err: %v", e.cmd.Process.Pid, err)
		}
		if err := proc.Kill(); err != nil {
			e.logger.Printf("[ERROR] can't kill process with pid: %v, err: %v", e.cmd.Process.Pid, err)
		}
	}

	if e.ctx.FSIsolation {
		if err := e.removeChrootMounts(); err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}
	if e.ctx.ResourceLimits {
		if err := e.destroyCgroup(); err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}
	return merr.ErrorOrNil()
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
