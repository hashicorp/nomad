package executor

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/armon/circbuf"
	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/lib/fifo"
	"github.com/hashicorp/nomad/client/stats"
	shelpers "github.com/hashicorp/nomad/helper/stats"

	"github.com/hashicorp/consul-template/signals"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

const (
	// ExecutorVersionLatest is the current and latest version of the executor
	ExecutorVersionLatest = "2.0.0"

	// ExecutorVersionPre0_9 is the version of executor use prior to the release
	// of 0.9.x
	ExecutorVersionPre0_9 = "1.1.0"
)

var (
	// The statistics the basic executor exposes
	ExecutorBasicMeasuredMemStats = []string{"RSS", "Swap"}
	ExecutorBasicMeasuredCpuStats = []string{"System Mode", "User Mode", "Percent"}
)

// Executor is the interface which allows a driver to launch and supervise
// a process
type Executor interface {
	Launch(*ExecCommand) (*ProcessState, error)
	Wait() (*ProcessState, error)
	Shutdown(string, time.Duration) error
	UpdateResources(*Resources) error
	Version() (*ExecutorVersion, error)
	Stats() (*cstructs.TaskResourceUsage, error)
	Signal(os.Signal) error
	Exec(deadline time.Time, cmd string, args []string) ([]byte, int, error)
}

// Resources describes the resource isolation required
type Resources struct {
	CPU      int
	MemoryMB int
	DiskMB   int
	IOPS     int
}

// ExecCommand holds the user command, args, and other isolation related
// settings.
type ExecCommand struct {
	// Cmd is the command that the user wants to run.
	Cmd string

	// Args is the args of the command that the user wants to run.
	Args []string

	// Resources defined by the task
	Resources *Resources

	// StdoutPath is the path the procoess stdout should be written to
	StdoutPath string
	stdout     io.WriteCloser

	// StderrPath is the path the procoess stderr should be written to
	StderrPath string
	stderr     io.WriteCloser

	// Env is the list of KEY=val pairs of environment variables to be set
	Env []string

	// User is the user which the executor uses to run the command.
	User string

	// TaskDir is the directory path on the host where for the task
	TaskDir string

	// ResourceLimits determines whether resource limits are enforced by the
	// executor.
	ResourceLimits bool

	// Cgroup marks whether we put the process in a cgroup. Setting this field
	// doesn't enforce resource limits. To enforce limits, set ResourceLimits.
	// Using the cgroup does allow more precise cleanup of processes.
	BasicProcessCgroup bool
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

// Stdout returns a writer for the configured file descriptor
func (c *ExecCommand) Stdout() (io.WriteCloser, error) {
	if c.stdout == nil {
		if c.StderrPath != "" {
			f, err := fifo.Open(c.StdoutPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create stdout: %v", err)
			}
			c.stdout = f
		} else {
			c.stdout = nopCloser{ioutil.Discard}
		}
	}
	return c.stdout, nil
}

// Stderr returns a writer for the configured file descriptor
func (c *ExecCommand) Stderr() (io.WriteCloser, error) {
	if c.stderr == nil {
		if c.StderrPath != "" {
			f, err := fifo.Open(c.StderrPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create stderr: %v", err)
			}
			c.stderr = f
		} else {
			c.stderr = nopCloser{ioutil.Discard}
		}
	}
	return c.stderr, nil
}

func (c *ExecCommand) Close() {
	stdout, err := c.Stdout()
	if err == nil {
		stdout.Close()
	}
	stderr, err := c.Stderr()
	if err == nil {
		stderr.Close()
	}
}

// ProcessState holds information about the state of a user process.
type ProcessState struct {
	Pid      int
	ExitCode int
	Signal   int
	Time     time.Time
}

// ExecutorVersion is the version of the executor
type ExecutorVersion struct {
	Version string
}

func (v *ExecutorVersion) GoString() string {
	return v.Version
}

// UniversalExecutor is an implementation of the Executor which launches and
// supervises processes. In addition to process supervision it provides resource
// and file system isolation
type UniversalExecutor struct {
	cmd     exec.Cmd
	command *ExecCommand

	exitState     *ProcessState
	processExited chan interface{}

	resConCtx resourceContainerContext

	totalCpuStats  *stats.CpuStats
	userCpuStats   *stats.CpuStats
	systemCpuStats *stats.CpuStats
	pidCollector   *pidCollector

	logger hclog.Logger
}

// NewExecutor returns an Executor
func NewExecutor(logger hclog.Logger) Executor {
	logger = logger.Named("executor")
	if err := shelpers.Init(); err != nil {
		logger.Error("unable to initialize stats", "error", err)
	}
	return &UniversalExecutor{
		logger:         logger,
		processExited:  make(chan interface{}),
		totalCpuStats:  stats.NewCpuStats(),
		userCpuStats:   stats.NewCpuStats(),
		systemCpuStats: stats.NewCpuStats(),
		pidCollector:   newPidCollector(logger),
	}
}

// Version returns the api version of the executor
func (e *UniversalExecutor) Version() (*ExecutorVersion, error) {
	return &ExecutorVersion{Version: ExecutorVersionLatest}, nil
}

// Launch launches the main process and returns its state. It also
// configures an applies isolation on certain platforms.
func (e *UniversalExecutor) Launch(command *ExecCommand) (*ProcessState, error) {
	e.logger.Info("launching command", "command", command.Cmd, "args", strings.Join(command.Args, " "))

	e.command = command

	// setting the user of the process
	if command.User != "" {
		e.logger.Debug("running command as user", "user", command.User)
		if err := e.runAs(command.User); err != nil {
			return nil, err
		}
	}

	// set the task dir as the working directory for the command
	e.cmd.Dir = e.command.TaskDir

	// start command in separate process group
	if err := e.setNewProcessGroup(); err != nil {
		return nil, err
	}

	// Setup cgroups on linux
	if err := e.configureResourceContainer(os.Getpid()); err != nil {
		return nil, err
	}

	stdout, err := e.command.Stdout()
	if err != nil {
		return nil, err
	}
	stderr, err := e.command.Stderr()
	if err != nil {
		return nil, err
	}

	e.cmd.Stdout = stdout
	e.cmd.Stderr = stderr

	// Look up the binary path and make it executable
	absPath, err := e.lookupBin(command.Cmd)
	if err != nil {
		return nil, err
	}

	if err := e.makeExecutable(absPath); err != nil {
		return nil, err
	}

	path := absPath

	// Set the commands arguments
	e.cmd.Path = path
	e.cmd.Args = append([]string{e.cmd.Path}, command.Args...)
	e.cmd.Env = e.command.Env

	// Start the process
	if err := e.cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command path=%q --- args=%q: %v", path, e.cmd.Args, err)
	}

	go e.pidCollector.collectPids(e.processExited, getAllPids)
	go e.wait()
	return &ProcessState{Pid: e.cmd.Process.Pid, ExitCode: -1, Time: time.Now()}, nil
}

// runAs takes a user id as a string and looks up the user, and sets the command
// to execute as that user.
func (e *UniversalExecutor) runAs(userid string) error {
	u, err := user.Lookup(userid)
	if err != nil {
		return fmt.Errorf("Failed to identify user %v: %v", userid, err)
	}

	// Get the groups the user is a part of
	gidStrings, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("Unable to lookup user's group membership: %v", err)
	}

	gids := make([]uint32, len(gidStrings))
	for _, gidString := range gidStrings {
		u, err := strconv.Atoi(gidString)
		if err != nil {
			return fmt.Errorf("Unable to convert user's group to int %s: %v", gidString, err)
		}

		gids = append(gids, uint32(u))
	}

	// Convert the uid and gid
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert userid to uint32: %s", err)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert groupid to uint32: %s", err)
	}

	// Set the command to run as that user and group.
	if e.cmd.SysProcAttr == nil {
		e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if e.cmd.SysProcAttr.Credential == nil {
		e.cmd.SysProcAttr.Credential = &syscall.Credential{}
	}
	e.cmd.SysProcAttr.Credential.Uid = uint32(uid)
	e.cmd.SysProcAttr.Credential.Gid = uint32(gid)
	e.cmd.SysProcAttr.Credential.Groups = gids

	e.logger.Debug("setting process user", "user", uid, "group", gid, "additional_groups", gids)

	return nil
}

// Exec a command inside a container for exec and java drivers.
func (e *UniversalExecutor) Exec(deadline time.Time, name string, args []string) ([]byte, int, error) {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	return ExecScript(ctx, e.cmd.Dir, e.command.Env, e.cmd.SysProcAttr, name, args)
}

// ExecScript executes cmd with args and returns the output, exit code, and
// error. Output is truncated to client/driver/structs.CheckBufSize
func ExecScript(ctx context.Context, dir string, env []string, attrs *syscall.SysProcAttr,
	name string, args []string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	// Copy runtime environment from the main command
	cmd.SysProcAttr = attrs
	cmd.Dir = dir
	cmd.Env = env

	// Capture output
	buf, _ := circbuf.NewBuffer(int64(dstructs.CheckBufSize))
	cmd.Stdout = buf
	cmd.Stderr = buf

	if err := cmd.Run(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			// Non-exit error, return it and let the caller treat
			// it as a critical failure
			return nil, 0, err
		}

		// Some kind of error happened; default to critical
		exitCode := 2
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
		}

		// Don't return the exitError as the caller only needs the
		// output and code.
		return buf.Bytes(), exitCode, nil
	}
	return buf.Bytes(), 0, nil
}

// Wait waits until a process has exited and returns it's exitcode and errors
func (e *UniversalExecutor) Wait() (*ProcessState, error) {
	<-e.processExited
	return e.exitState, nil
}

func (e *UniversalExecutor) UpdateResources(resources *Resources) error {
	return nil
}

func (e *UniversalExecutor) wait() {
	defer close(e.processExited)
	err := e.cmd.Wait()
	if err == nil {
		e.exitState = &ProcessState{Pid: 0, ExitCode: 0, Time: time.Now()}
		return
	}

	e.command.Close()

	exitCode := 1
	var signal int
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
			if status.Signaled() {
				// bash(1) uses the lower 7 bits of a uint8
				// to indicate normal program failure (see
				// <sysexits.h>). If a process terminates due
				// to a signal, encode the signal number to
				// indicate which signal caused the process
				// to terminate.  Mirror this exit code
				// encoding scheme.
				const exitSignalBase = 128
				signal = int(status.Signal())
				exitCode = exitSignalBase + signal
			}
		}
	} else {
		e.logger.Warn("unexpected Cmd.Wait() error type", "error", err)
	}

	e.exitState = &ProcessState{Pid: 0, ExitCode: exitCode, Signal: signal, Time: time.Now()}
}

var (
	// finishedErr is the error message received when trying to kill and already
	// exited process.
	finishedErr = "os: process already finished"

	// noSuchProcessErr is the error message received when trying to kill a non
	// existing process (e.g. when killing a process group).
	noSuchProcessErr = "no such process"
)

// Exit cleans up the alloc directory, destroys resource container and kills the
// user process
func (e *UniversalExecutor) Shutdown(signal string, grace time.Duration) error {
	var merr multierror.Error

	// If the executor did not launch a process, return.
	if e.command == nil {
		return nil
	}

	if e.cmd.Process == nil {
		return fmt.Errorf("executor failed to shutdown error: no process found")
	}

	if signal == "" {
		signal = "SIGINT"
	}

	sig, ok := signals.SignalLookup[signal]
	if !ok {
		return fmt.Errorf("error unknown signal given for shutdown: %s", signal)
	}

	proc, err := os.FindProcess(e.cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("executor failed to find process: %v", err)
	}

	if err := e.shutdownProcess(sig, proc); err != nil {
		return err
	}

	select {
	case <-e.processExited:
	case <-time.After(grace):
		proc.Kill()
	}

	// Prefer killing the process via the resource container.
	if !(e.command.ResourceLimits || e.command.BasicProcessCgroup) {
		if err := e.cleanupChildProcesses(proc); err != nil && err.Error() != finishedErr {
			merr.Errors = append(merr.Errors,
				fmt.Errorf("can't kill process with pid %d: %v", e.cmd.Process.Pid, err))
		}
	}

	if e.command.ResourceLimits || e.command.BasicProcessCgroup {
		if err := e.resConCtx.executorCleanup(); err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}

	return merr.ErrorOrNil()
}

// lookupBin looks for path to the binary to run by looking for the binary in
// the following locations, in-order: task/local/, task/, based on host $PATH.
// The return path is absolute.
func (e *UniversalExecutor) lookupBin(bin string) (string, error) {
	// Check in the local directory
	local := filepath.Join(e.command.TaskDir, allocdir.TaskLocal, bin)
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	// Check at the root of the task's directory
	root := filepath.Join(e.command.TaskDir, bin)
	if _, err := os.Stat(root); err == nil {
		return root, nil
	}

	// Check the $PATH
	if host, err := exec.LookPath(bin); err == nil {
		return host, nil
	}

	return "", fmt.Errorf("binary %q could not be found", bin)
}

// makeExecutable makes the given file executable for root,group,others.
func (e *UniversalExecutor) makeExecutable(binPath string) error {
	if runtime.GOOS == "windows" {
		return nil
	}

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

// Signal sends the passed signal to the task
func (e *UniversalExecutor) Signal(s os.Signal) error {
	if e.cmd.Process == nil {
		return fmt.Errorf("Task not yet run")
	}

	e.logger.Debug("sending signal to PID", "signal", s, "pid", e.cmd.Process.Pid)
	err := e.cmd.Process.Signal(s)
	if err != nil {
		e.logger.Error("sending signal failed", "signal", s, "error", err)
		return err
	}

	return nil
}

func (e *UniversalExecutor) Stats() (*cstructs.TaskResourceUsage, error) {
	pidStats, err := e.pidCollector.pidStats()
	if err != nil {
		return nil, err
	}
	return aggregatedResourceUsage(e.systemCpuStats, pidStats), nil
}
