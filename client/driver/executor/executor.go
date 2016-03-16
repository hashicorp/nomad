package executor

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/driver/logging"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
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

	// LogConfig provides the configuration related to log rotation
	LogConfig *structs.LogConfig
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
	IsolationConfig *cstructs.IsolationConfig
	Time            time.Time
}

// Executor is the interface which allows a driver to launch and supervise
// a process
type Executor interface {
	LaunchCmd(command *ExecCommand, ctx *ExecutorContext) (*ProcessState, error)
	Wait() (*ProcessState, error)
	ShutDown() error
	Exit() error
	UpdateLogConfig(logConfig *structs.LogConfig) error
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
	lre           *logging.FileRotator
	lro           *logging.FileRotator

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
	e.logger.Printf("[DEBUG] executor: launching command %v %v", command.Cmd, strings.Join(command.Args, " "))

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

	logFileSize := int64(ctx.LogConfig.MaxFileSizeMB * 1024 * 1024)
	lro, err := logging.NewFileRotator(ctx.AllocDir.LogDir(), fmt.Sprintf("%v.stdout", ctx.TaskName),
		ctx.LogConfig.MaxFiles, logFileSize, e.logger)

	if err != nil {
		return nil, fmt.Errorf("error creating log rotator for stdout of task %v", err)
	}
	e.cmd.Stdout = lro
	e.lro = lro

	lre, err := logging.NewFileRotator(ctx.AllocDir.LogDir(), fmt.Sprintf("%v.stderr", ctx.TaskName),
		ctx.LogConfig.MaxFiles, logFileSize, e.logger)
	if err != nil {
		return nil, fmt.Errorf("error creating log rotator for stderr of task %v", err)
	}
	e.cmd.Stderr = lre
	e.lre = lre

	// setting the env, path and args for the command
	e.ctx.TaskEnv.Build()
	e.cmd.Env = ctx.TaskEnv.EnvList()
	e.cmd.Path = ctx.TaskEnv.ReplaceEnv(command.Cmd)
	e.cmd.Args = append([]string{e.cmd.Path}, ctx.TaskEnv.ParseAndReplace(command.Args)...)

	// Ensure that the binary being started is executable.
	if err := e.makeExecutable(e.cmd.Path); err != nil {
		return nil, err
	}

	// starting the process
	if err := e.cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting command: %v", err)
	}
	go e.wait()
	ic := &cstructs.IsolationConfig{Cgroup: e.groups}
	return &ProcessState{Pid: e.cmd.Process.Pid, ExitCode: -1, IsolationConfig: ic, Time: time.Now()}, nil
}

// Wait waits until a process has exited and returns it's exitcode and errors
func (e *UniversalExecutor) Wait() (*ProcessState, error) {
	<-e.processExited
	return e.exitState, nil
}

// UpdateLogConfig updates the log configuration
func (e *UniversalExecutor) UpdateLogConfig(logConfig *structs.LogConfig) error {
	e.ctx.LogConfig = logConfig
	if e.lro == nil {
		return fmt.Errorf("log rotator for stdout doesn't exist")
	}
	e.lro.MaxFiles = logConfig.MaxFiles
	e.lro.FileSize = int64(logConfig.MaxFileSizeMB * 1024 * 1024)

	if e.lre == nil {
		return fmt.Errorf("log rotator for stderr doesn't exist")
	}
	e.lre.MaxFiles = logConfig.MaxFiles
	e.lre.FileSize = int64(logConfig.MaxFileSizeMB * 1024 * 1024)
	return nil
}

func (e *UniversalExecutor) wait() {
	defer close(e.processExited)
	err := e.cmd.Wait()
	e.lre.Close()
	e.lro.Close()
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
		e.lock.Lock()
		DestroyCgroup(e.groups)
		e.lock.Unlock()
	}
	e.exitState = &ProcessState{Pid: 0, ExitCode: exitCode, Time: time.Now()}
}

var (
	// finishedErr is the error message received when trying to kill and already
	// exited process.
	finishedErr = "os: process already finished"
)

// Exit cleans up the alloc directory, destroys cgroups and kills the user
// process
func (e *UniversalExecutor) Exit() error {
	var merr multierror.Error
	if e.cmd.Process != nil {
		proc, err := os.FindProcess(e.cmd.Process.Pid)
		if err != nil {
			e.logger.Printf("[ERR] executor: can't find process with pid: %v, err: %v",
				e.cmd.Process.Pid, err)
		} else if err := proc.Kill(); err != nil && err.Error() != finishedErr {
			merr.Errors = append(merr.Errors,
				fmt.Errorf("can't kill process with pid: %v, err: %v", e.cmd.Process.Pid, err))
		}
	}

	if e.ctx.FSIsolation {
		if err := e.removeChrootMounts(); err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}
	if e.ctx.ResourceLimits {
		e.lock.Lock()
		if err := DestroyCgroup(e.groups); err != nil {
			merr.Errors = append(merr.Errors, err)
		}
		e.lock.Unlock()
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

// configureTaskDir sets the task dir in the executor
func (e *UniversalExecutor) configureTaskDir() error {
	taskDir, ok := e.ctx.AllocDir.TaskDirs[e.ctx.TaskName]
	e.taskDir = taskDir
	if !ok {
		return fmt.Errorf("couldn't find task directory for task %v", e.ctx.TaskName)
	}
	e.cmd.Dir = taskDir
	return nil
}

// makeExecutablePosix makes the given file executable for root,group,others.
func (e *UniversalExecutor) makeExecutablePosix(binPath string) error {
	fi, err := os.Stat(binPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("binary %q does not exist", binPath)
		}
		return fmt.Errorf("specified binary is invalid: %v", err)
	}

	// If it is not executable, make it so.
	perm := fi.Mode().Perm()
	req := os.FileMode(0555)
	if perm&req != req {
		if err := os.Chmod(binPath, perm|req); err != nil {
			return fmt.Errorf("error making %q executable: %s", binPath, err)
		}
	}
	return nil
}
