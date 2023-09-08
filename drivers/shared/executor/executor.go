// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/armon/circbuf"
	"github.com/creack/pty"
	"github.com/hashicorp/consul-template/signals"
	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	"github.com/hashicorp/nomad/client/lib/fifo"
	"github.com/hashicorp/nomad/client/lib/numalib"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/shared/executor/procstats"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/syndtr/gocapability/capability"
)

const (
	// ExecutorVersionLatest is the current and latest version of the executor
	ExecutorVersionLatest = "2.0.0"

	// ExecutorVersionPre0_9 is the version of executor use prior to the release
	// of 0.9.x
	ExecutorVersionPre0_9 = "1.1.0"

	// IsolationModePrivate represents the private isolation mode for a namespace
	IsolationModePrivate = "private"

	// IsolationModeHost represents the host isolation mode for a namespace
	IsolationModeHost = "host"
)

var (
	// The statistics the basic executor exposes
	ExecutorBasicMeasuredMemStats = []string{"RSS", "Swap"}
	ExecutorBasicMeasuredCpuStats = []string{"System Mode", "User Mode", "Percent"}
)

// Executor is the interface which allows a driver to launch and supervise
// a process
type Executor interface {
	// Launch a user process configured by the given ExecCommand
	Launch(launchCmd *ExecCommand) (*ProcessState, error)

	// Wait blocks until the process exits or an error occures
	Wait(ctx context.Context) (*ProcessState, error)

	// Shutdown will shutdown the executor by stopping the user process,
	// cleaning up and resources created by the executor. The shutdown sequence
	// will first send the given signal to the process. This defaults to "SIGINT"
	// if not specified. The executor will then wait for the process to exit
	// before cleaning up other resources. If the executor waits longer than the
	// given grace period, the process is forcefully killed.
	//
	// To force kill the user process, gracePeriod can be set to 0.
	Shutdown(signal string, gracePeriod time.Duration) error

	// UpdateResources updates any resource isolation enforcement with new
	// constraints if supported.
	UpdateResources(*drivers.Resources) error

	// Version returns the executor API version
	Version() (*ExecutorVersion, error)

	// Returns a channel of stats. Stats are collected and
	// pushed to the channel on the given interval
	Stats(context.Context, time.Duration) (<-chan *cstructs.TaskResourceUsage, error)

	// Signal sends the given signal to the user process
	Signal(os.Signal) error

	// Exec executes the given command and args inside the executor context
	// and returns the output and exit code.
	Exec(deadline time.Time, cmd string, args []string) ([]byte, int, error)

	ExecStreaming(ctx context.Context, cmd []string, tty bool,
		stream drivers.ExecTaskStream) error
}

// ExecCommand holds the user command, args, and other isolation related
// settings.
//
// Important (!): when adding fields, make sure to update the RPC methods in
// grpcExecutorClient.Launch and grpcExecutorServer.Launch. Number of hours
// spent tracking this down: too many.
type ExecCommand struct {
	// Cmd is the command that the user wants to run.
	Cmd string

	// Args is the args of the command that the user wants to run.
	Args []string

	// Resources defined by the task
	Resources *drivers.Resources

	// StdoutPath is the path the process stdout should be written to
	StdoutPath string
	stdout     io.WriteCloser

	// StderrPath is the path the process stderr should be written to
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

	// NoPivotRoot disables using pivot_root for isolation, useful when the root
	// partition is on a ramdisk which does not support pivot_root,
	// see man 2 pivot_root
	NoPivotRoot bool

	// Mounts are the host paths to be be made available inside rootfs
	Mounts []*drivers.MountConfig

	// Devices are the the device nodes to be created in isolation environment
	Devices []*drivers.DeviceConfig

	// NetworkIsolation is the network isolation configuration.
	NetworkIsolation *drivers.NetworkIsolationSpec

	// ModePID is the PID isolation mode (private or host).
	ModePID string

	// ModeIPC is the IPC isolation mode (private or host).
	ModeIPC string

	// Capabilities are the linux capabilities to be enabled by the task driver.
	Capabilities []string
}

// Cgroup returns the path to the cgroup the Nomad client is managing for the
// task that is about to be run.
//
// On cgroups v1 systems this returns the path to the cpuset cgroup specifically.
//
// On cgroups v2 systems this returns the patah to the task's scope.
//
// On non-Linux systems this returns the empty string and has no meaning.
func (c *ExecCommand) Cgroup() string {
	if c == nil || c.Resources == nil || c.Resources.LinuxResources == nil {
		return ""
	}
	return c.Resources.LinuxResources.CpusetCgroupPath
}

// SetWriters sets the writer for the process stdout and stderr. This should
// not be used if writing to a file path such as a fifo file. SetStdoutWriter
// is mainly used for unit testing purposes.
func (c *ExecCommand) SetWriters(out io.WriteCloser, err io.WriteCloser) {
	c.stdout = out
	c.stderr = err
}

// GetWriters returns the unexported io.WriteCloser for the stdout and stderr
// handles. This is mainly used for unit testing purposes.
func (c *ExecCommand) GetWriters() (stdout io.WriteCloser, stderr io.WriteCloser) {
	return c.stdout, c.stderr
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

// Stdout returns a writer for the configured file descriptor
func (c *ExecCommand) Stdout() (io.WriteCloser, error) {
	if c.stdout == nil {
		if c.StdoutPath != "" && c.StdoutPath != os.DevNull {
			f, err := fifo.OpenWriter(c.StdoutPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create stdout: %v", err)
			}
			c.stdout = f
		} else {
			c.stdout = nopCloser{io.Discard}
		}
	}
	return c.stdout, nil
}

// Stderr returns a writer for the configured file descriptor
func (c *ExecCommand) Stderr() (io.WriteCloser, error) {
	if c.stderr == nil {
		if c.StderrPath != "" && c.StderrPath != os.DevNull {
			f, err := fifo.OpenWriter(c.StderrPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create stderr: %v", err)
			}
			c.stderr = f
		} else {
			c.stderr = nopCloser{io.Discard}
		}
	}
	return c.stderr, nil
}

func (c *ExecCommand) Close() {
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.stderr != nil {
		c.stderr.Close()
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
	childCmd exec.Cmd
	command  *ExecCommand

	exitState     *ProcessState
	processExited chan interface{}

	top            cpustats.Topology
	totalCpuStats  *cpustats.Tracker
	userCpuStats   *cpustats.Tracker
	systemCpuStats *cpustats.Tracker
	processStats   procstats.ProcessStats

	logger hclog.Logger
}

// NewExecutor returns an Executor
func NewExecutor(logger hclog.Logger) Executor {
	top := numalib.Scan(numalib.PlatformScanners()) // TODO(shoenig) grpc plumbing
	ue := &UniversalExecutor{
		logger:         logger.Named("executor"),
		top:            top,
		processExited:  make(chan interface{}),
		totalCpuStats:  cpustats.New(top),
		userCpuStats:   cpustats.New(top),
		systemCpuStats: cpustats.New(top),
	}
	ue.processStats = procstats.New(top, ue)
	return ue
}

// Version returns the api version of the executor
func (e *UniversalExecutor) Version() (*ExecutorVersion, error) {
	return &ExecutorVersion{Version: ExecutorVersionLatest}, nil
}

// Launch launches the main process and returns its state. It also
// configures an applies isolation on certain platforms.
func (e *UniversalExecutor) Launch(command *ExecCommand) (*ProcessState, error) {
	e.logger.Trace("preparing to launch command", "command", command.Cmd, "args", strings.Join(command.Args, " "))

	e.command = command

	// setting the user of the process
	if command.User != "" {
		e.logger.Debug("running command as user", "user", command.User)
		if err := setCmdUser(&e.childCmd, command.User); err != nil {
			return nil, err
		}
	}

	// set the task dir as the working directory for the command
	e.childCmd.Dir = e.command.TaskDir

	// start command in separate process group
	if err := e.setNewProcessGroup(); err != nil {
		return nil, err
	}

	// setup containment (i.e. cgroups on linux)
	if cleanup, err := e.configureResourceContainer(command, os.Getpid()); err != nil {
		// keep going; some folks run nomad as non-root and expect this driver to still work
		e.logger.Warn("failed to configure container, process isolation will not work", "error", err)
	} else {
		defer cleanup()
	}

	stdout, err := e.command.Stdout()
	if err != nil {
		return nil, err
	}
	stderr, err := e.command.Stderr()
	if err != nil {
		return nil, err
	}

	e.childCmd.Stdout = stdout
	e.childCmd.Stderr = stderr

	// Look up the binary path and make it executable
	absPath, err := lookupBin(command.TaskDir, command.Cmd)
	if err != nil {
		return nil, err
	}

	if err := makeExecutable(absPath); err != nil {
		return nil, err
	}

	path := absPath

	// Set the commands arguments
	e.childCmd.Path = path
	e.childCmd.Args = append([]string{e.childCmd.Path}, command.Args...)
	e.childCmd.Env = e.command.Env

	// Start the process
	if err = withNetworkIsolation(e.childCmd.Start, command.NetworkIsolation); err != nil {
		return nil, fmt.Errorf("failed to start command path=%q --- args=%q: %v", path, e.childCmd.Args, err)
	}

	go e.wait()
	return &ProcessState{Pid: e.childCmd.Process.Pid, ExitCode: -1, Time: time.Now()}, nil
}

// Exec a command inside a container for exec and java drivers.
func (e *UniversalExecutor) Exec(deadline time.Time, name string, args []string) ([]byte, int, error) {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	if cleanup, err := e.setSubCmdCgroup(&e.childCmd, e.command.Cgroup()); err != nil {
		return nil, 0, err
	} else {
		defer cleanup()
	}

	return ExecScript(ctx, e.childCmd.Dir, e.command.Env, e.childCmd.SysProcAttr, e.command.NetworkIsolation, name, args)
}

// ExecScript executes cmd with args and returns the output, exit code, and
// error. Output is truncated to drivers/shared/structs.CheckBufSize
func ExecScript(ctx context.Context, dir string, env []string, attrs *syscall.SysProcAttr,
	netSpec *drivers.NetworkIsolationSpec, name string, args []string) ([]byte, int, error) {

	cmd := exec.CommandContext(ctx, name, args...)

	// Copy runtime environment from the main command
	cmd.SysProcAttr = attrs

	cmd.Dir = dir
	cmd.Env = env

	// Capture output
	buf, _ := circbuf.NewBuffer(int64(drivers.CheckBufSize))
	cmd.Stdout = buf
	cmd.Stderr = buf

	if err := withNetworkIsolation(cmd.Run, netSpec); err != nil {
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

func (e *UniversalExecutor) ExecStreaming(ctx context.Context, command []string, tty bool,
	stream drivers.ExecTaskStream) error {

	if len(command) == 0 {
		return fmt.Errorf("command is required")
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	cmd.Dir = "/"
	cmd.Env = e.childCmd.Env

	execHelper := &execHelper{
		logger: e.logger,

		newTerminal: func() (func() (*os.File, error), *os.File, error) {
			pty, tty, err := pty.Open()
			if err != nil {
				return nil, nil, err
			}

			return func() (*os.File, error) { return pty, nil }, tty, err
		},
		setTTY: func(tty *os.File) error {
			cmd.SysProcAttr = sessionCmdAttr(tty)

			cmd.Stdin = tty
			cmd.Stdout = tty
			cmd.Stderr = tty
			return nil
		},
		setIO: func(stdin io.Reader, stdout, stderr io.Writer) error {
			cmd.Stdin = stdin
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			return nil
		},
		processStart: func() error {
			if u := e.command.User; u != "" {
				if err := setCmdUser(cmd, u); err != nil {
					return err
				}
			}
			cgroup := e.command.Cgroup()
			if cleanup, err := e.setSubCmdCgroup(cmd, cgroup); err != nil {
				return err
			} else {
				defer cleanup()
			}
			return withNetworkIsolation(cmd.Start, e.command.NetworkIsolation)
		},
		processWait: func() (*os.ProcessState, error) {
			err := cmd.Wait()
			return cmd.ProcessState, err
		},
	}

	return execHelper.run(ctx, tty, stream)
}

// Wait waits until a process has exited and returns it's exitcode and errors
func (e *UniversalExecutor) Wait(ctx context.Context) (*ProcessState, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.processExited:
		return e.exitState, nil
	}
}

func (e *UniversalExecutor) UpdateResources(resources *drivers.Resources) error {
	return nil
}

func (e *UniversalExecutor) wait() {
	defer close(e.processExited)
	defer e.command.Close()
	pid := e.childCmd.Process.Pid
	err := e.childCmd.Wait()
	if err == nil {
		e.exitState = &ProcessState{Pid: pid, ExitCode: 0, Time: time.Now()}
		return
	}

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

	e.exitState = &ProcessState{Pid: pid, ExitCode: exitCode, Signal: signal, Time: time.Now()}
}

var (
	// finishedErr is the error message received when trying to kill and already
	// exited process.
	finishedErr = "os: process already finished"

	// noSuchProcessErr is the error message received when trying to kill a non
	// existing process (e.g. when killing a process group).
	noSuchProcessErr = "no such process"
)

// Shutdown cleans up the alloc directory, destroys resource container and
// kills the user process.
func (e *UniversalExecutor) Shutdown(signal string, grace time.Duration) error {
	e.logger.Debug("shutdown requested", "signal", signal, "grace_period_ms", grace.Round(time.Millisecond))
	var merr multierror.Error

	// If the executor did not launch a process, return.
	if e.command == nil {
		return nil
	}

	// If there is no process we can't shutdown
	if e.childCmd.Process == nil {
		e.logger.Warn("failed to shutdown due to missing process", "error", "no process found")
		return fmt.Errorf("executor failed to shutdown error: no process found")
	}

	proc, err := os.FindProcess(e.childCmd.Process.Pid)
	if err != nil {
		err = fmt.Errorf("executor failed to find process: %v", err)
		e.logger.Warn("failed to shutdown due to inability to find process", "pid", e.childCmd.Process.Pid, "error", err)
		return err
	}

	// If grace is 0 then skip shutdown logic
	if grace > 0 {
		// Default signal to SIGINT if not set
		if signal == "" {
			signal = "SIGINT"
		}

		sig, ok := signals.SignalLookup[signal]
		if !ok {
			err = fmt.Errorf("error unknown signal given for shutdown: %s", signal)
			e.logger.Warn("failed to shutdown", "error", err)
			return err
		}

		if err := e.shutdownProcess(sig, proc); err != nil {
			e.logger.Warn("failed to shutdown process", "pid", proc.Pid, "error", err)
			return err
		}

		select {
		case <-e.processExited:
		case <-time.After(grace):
			proc.Kill()
		}
	} else {
		proc.Kill()
	}

	// Issue sigkill to the process group (if possible)
	if err = e.killProcessTree(proc); err != nil {
		e.logger.Warn("failed to shutdown process group", "pid", proc.Pid, "error", err)
	}

	// Wait for process to exit
	select {
	case <-e.processExited:
	case <-time.After(time.Second * 15):
		e.logger.Warn("process did not exit after 15 seconds")
		merr.Errors = append(merr.Errors, fmt.Errorf("process did not exit after 15 seconds"))
	}

	if err = merr.ErrorOrNil(); err != nil {
		// Note that proclib in the TR shutdown may also dispatch a final platform
		// cleanup technique (e.g. cgroup kill), but if we get to the point where
		// that matters the Task was doing something naughty.
		e.logger.Warn("failed to shutdown due to some error", "error", err.Error())
		return err
	}

	return nil
}

// Signal sends the passed signal to the task
func (e *UniversalExecutor) Signal(s os.Signal) error {
	if e.childCmd.Process == nil {
		return fmt.Errorf("Task not yet run")
	}

	e.logger.Debug("sending signal to PID", "signal", s, "pid", e.childCmd.Process.Pid)
	err := e.childCmd.Process.Signal(s)
	if err != nil {
		e.logger.Error("sending signal failed", "signal", s, "error", err)
		return err
	}

	return nil
}

func (e *UniversalExecutor) Stats(ctx context.Context, interval time.Duration) (<-chan *cstructs.TaskResourceUsage, error) {
	ch := make(chan *cstructs.TaskResourceUsage)
	go e.handleStats(ch, ctx, interval)
	return ch, nil
}

func (e *UniversalExecutor) handleStats(ch chan *cstructs.TaskResourceUsage, ctx context.Context, interval time.Duration) {
	defer close(ch)
	timer := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			return

		case <-timer.C:
			timer.Reset(interval)
		}

		stats := e.processStats.StatProcesses()

		select {
		case <-ctx.Done():
			return
		case ch <- procstats.Aggregate(e.systemCpuStats, stats):
		}
	}
}

// lookupBin looks for path to the binary to run by looking for the binary in
// the following locations, in-order:
// task/local/, task/, on the host file system, in host $PATH
// The return path is absolute.
func lookupBin(taskDir string, bin string) (string, error) {
	// Check in the local directory
	local := filepath.Join(taskDir, allocdir.TaskLocal, bin)
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	// Check at the root of the task's directory
	root := filepath.Join(taskDir, bin)
	if _, err := os.Stat(root); err == nil {
		return root, nil
	}

	// when checking host paths, check with Stat first if path is absolute
	// as exec.LookPath only considers files already marked as executable
	// and only consider this for absolute paths to avoid depending on
	// current directory of nomad which may cause unexpected behavior
	if _, err := os.Stat(bin); err == nil && filepath.IsAbs(bin) {
		return bin, nil
	}

	// Check the $PATH
	if host, err := exec.LookPath(bin); err == nil {
		return host, nil
	}

	return "", fmt.Errorf("binary %q could not be found", bin)
}

// makeExecutable makes the given file executable for root,group,others.
func makeExecutable(binPath string) error {
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

// SupportedCaps returns a list of all supported capabilities in kernel.
func SupportedCaps(allowNetRaw bool) []string {
	var allCaps []string
	last := capability.CAP_LAST_CAP
	// workaround for RHEL6 which has no /proc/sys/kernel/cap_last_cap
	if last == capability.Cap(63) {
		last = capability.CAP_BLOCK_SUSPEND
	}
	for _, cap := range capability.List() {
		if cap > last {
			continue
		}
		if !allowNetRaw && cap == capability.CAP_NET_RAW {
			continue
		}
		allCaps = append(allCaps, fmt.Sprintf("CAP_%s", strings.ToUpper(cap.String())))
	}
	return allCaps
}
