package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/armon/circbuf"
	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	ps "github.com/mitchellh/go-ps"
	"github.com/shirou/gopsutil/process"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/logging"
	"github.com/hashicorp/nomad/client/stats"
	shelpers "github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/helper/uuid"

	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

const (
	// pidScanInterval is the interval at which the executor scans the process
	// tree for finding out the pids that the executor and it's child processes
	// have forked
	pidScanInterval = 5 * time.Second

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
	Kill() error
	Destroy() error
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
	stdout     *os.File

	// StderrPath is the path the procoess stderr should be written to
	StderrPath string
	stderr     *os.File

	// Env is the list of KEY=val pairs of environment variables to be set
	Env []string

	// TaskKillSignal is an optional field which signal to kill the process
	TaskKillSignal os.Signal

	// FSIsolation determines whether the command would be run in a chroot.
	FSIsolation bool

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

// Stdout returns a writer for the configured file descriptor
func (c *ExecCommand) Stdout() (*os.File, error) {
	if c.stdout == nil {
		f, err := os.Open(c.StdoutPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout: %v", err)
		}
		c.stdout = f
	}
	return c.stdout, nil
}

// Stderr returns a writer for the configured file descriptor
func (c *ExecCommand) Stderr() (*os.File, error) {
	if c.stderr == nil {
		f, err := os.Open(c.StderrPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create stderr: %v", err)
		}
		c.stderr = f
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
	Pid             int
	ExitCode        int
	Signal          int
	IsolationConfig *dstructs.IsolationConfig
	Time            time.Time
}

// nomadPid holds a pid and it's cpu percentage calculator
type nomadPid struct {
	pid           int
	cpuStatsTotal *stats.CpuStats
	cpuStatsUser  *stats.CpuStats
	cpuStatsSys   *stats.CpuStats
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

	pids                map[int]*nomadPid
	pidLock             sync.RWMutex
	exitState           *ProcessState
	processExited       chan interface{}
	fsIsolationEnforced bool

	syslogServer *logging.SyslogServer
	syslogChan   chan *logging.SyslogMessage

	totalCpuStats  *stats.CpuStats
	userCpuStats   *stats.CpuStats
	systemCpuStats *stats.CpuStats
	logger         hclog.Logger
}

// NewExecutor returns an Executor
func NewExecutor(logger hclog.Logger) Executor {
	logger = logger.Named("executor")
	if err := shelpers.Init(); err != nil {
		logger.Error("unable to initialize stats", "err", err)
	}

	var exec Executor

	// TODO: only use libcontainer on linux /w cgroups
	exec = newLibcontainerExecutor(logger)
	return exec
}

func newLibcontainerExecutor(logger hclog.Logger) Executor {
	return &LibcontainerExecutor{
		id:             strings.Replace(uuid.Generate(), "-", "_", 0),
		logger:         logger,
		totalCpuStats:  stats.NewCpuStats(),
		userCpuStats:   stats.NewCpuStats(),
		systemCpuStats: stats.NewCpuStats(),
	}
}

func newUniversalExecutor(logger hclog.Logger) *UniversalExecutor {
	return &UniversalExecutor{
		logger:         logger,
		processExited:  make(chan interface{}),
		totalCpuStats:  stats.NewCpuStats(),
		userCpuStats:   stats.NewCpuStats(),
		systemCpuStats: stats.NewCpuStats(),
		pids:           make(map[int]*nomadPid),
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

	// set the task dir as the working directory for the command
	e.cmd.Dir = e.command.TaskDir

	// start command in separate process group
	if err := e.setNewProcessGroup(); err != nil {
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

	// Determine the path to run as it may have to be relative to the chroot.
	if e.fsIsolationEnforced {
		rel, err := filepath.Rel(e.command.TaskDir, path)
		if err != nil {
			return nil, fmt.Errorf("failed to determine relative path base=%q target=%q: %v", e.command.TaskDir, path, err)
		}
		path = rel
	}

	// Set the commands arguments
	e.cmd.Path = path
	e.cmd.Args = append([]string{e.cmd.Path}, command.Args...)
	e.cmd.Env = e.command.Env

	// Start the process
	if err := e.cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command path=%q --- args=%q: %v", path, e.cmd.Args, err)
	}

	go e.collectPids()
	go e.wait()
	return &ProcessState{Pid: e.cmd.Process.Pid, ExitCode: -1, IsolationConfig: nil, Time: time.Now()}, nil
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
		e.exitState = &ProcessState{Pid: 0, ExitCode: 0, IsolationConfig: nil, Time: time.Now()}
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
		e.logger.Warn("unexpected Cmd.Wait() error type", "err", err)
	}

	e.exitState = &ProcessState{Pid: 0, ExitCode: exitCode, Signal: signal, IsolationConfig: nil, Time: time.Now()}
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
func (e *UniversalExecutor) Destroy() error {
	var merr multierror.Error

	// If the executor did not launch a process, return.
	if e.command == nil {
		return nil
	}

	// Prefer killing the process via the resource container.
	if e.cmd.Process != nil && !(e.command.ResourceLimits || e.command.BasicProcessCgroup) {
		proc, err := os.FindProcess(e.cmd.Process.Pid)
		if err != nil {
			e.logger.Error("can't find process", "pid", e.cmd.Process.Pid, "err", err)
		} else if err := e.cleanupChildProcesses(proc); err != nil && err.Error() != finishedErr {
			merr.Errors = append(merr.Errors,
				fmt.Errorf("can't kill process with pid %d: %v", e.cmd.Process.Pid, err))
		}
	}

	return merr.ErrorOrNil()
}

// Shutdown sends an interrupt signal to the user process
func (e *UniversalExecutor) Kill() error {
	if e.cmd.Process == nil {
		return fmt.Errorf("executor.shutdown error: no process found")
	}
	proc, err := os.FindProcess(e.cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("executor.shutdown failed to find process: %v", err)
	}
	return e.shutdownProcess(proc)
}

// pidStats returns the resource usage stats per pid
func (e *UniversalExecutor) pidStats() (map[string]*cstructs.ResourceUsage, error) {
	stats := make(map[string]*cstructs.ResourceUsage)
	e.pidLock.RLock()
	pids := make(map[int]*nomadPid, len(e.pids))
	for k, v := range e.pids {
		pids[k] = v
	}
	e.pidLock.RUnlock()
	for pid, np := range pids {
		p, err := process.NewProcess(int32(pid))
		if err != nil {
			e.logger.Trace("unable to create new process", "pid", pid)
			continue
		}
		ms := &cstructs.MemoryStats{}
		if memInfo, err := p.MemoryInfo(); err == nil {
			ms.RSS = memInfo.RSS
			ms.Swap = memInfo.Swap
			ms.Measured = ExecutorBasicMeasuredMemStats
		}

		cs := &cstructs.CpuStats{}
		if cpuStats, err := p.Times(); err == nil {
			cs.SystemMode = np.cpuStatsSys.Percent(cpuStats.System * float64(time.Second))
			cs.UserMode = np.cpuStatsUser.Percent(cpuStats.User * float64(time.Second))
			cs.Measured = ExecutorBasicMeasuredCpuStats

			// calculate cpu usage percent
			cs.Percent = np.cpuStatsTotal.Percent(cpuStats.Total() * float64(time.Second))
		}
		stats[strconv.Itoa(pid)] = &cstructs.ResourceUsage{MemoryStats: ms, CpuStats: cs}
	}

	return stats, nil
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

// collectPids collects the pids of the child processes that the executor is
// running every 5 seconds
func (e *UniversalExecutor) collectPids() {
	// Fire the timer right away when the executor starts from there on the pids
	// are collected every scan interval
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			pids, err := e.getAllPids()
			if err != nil {
				e.logger.Debug("error collecting pids", "err", err)
			}
			e.pidLock.Lock()

			// Adding pids which are not being tracked
			for pid, np := range pids {
				if _, ok := e.pids[pid]; !ok {
					e.pids[pid] = np
				}
			}
			// Removing pids which are no longer present
			for pid := range e.pids {
				if _, ok := pids[pid]; !ok {
					delete(e.pids, pid)
				}
			}
			e.pidLock.Unlock()
			timer.Reset(pidScanInterval)
		case <-e.processExited:
			return
		}
	}
}

// scanPids scans all the pids on the machine running the current executor and
// returns the child processes of the executor.
func (e *UniversalExecutor) scanPids(parentPid int, allPids []ps.Process) (map[int]*nomadPid, error) {
	processFamily := make(map[int]struct{})
	processFamily[parentPid] = struct{}{}

	// A mapping of pids to their parent pids. It is used to build the process
	// tree of the executing task
	pidsRemaining := make(map[int]int, len(allPids))
	for _, pid := range allPids {
		pidsRemaining[pid.Pid()] = pid.PPid()
	}

	for {
		// flag to indicate if we have found a match
		foundNewPid := false

		for pid, ppid := range pidsRemaining {
			_, childPid := processFamily[ppid]

			// checking if the pid is a child of any of the parents
			if childPid {
				processFamily[pid] = struct{}{}
				delete(pidsRemaining, pid)
				foundNewPid = true
			}
		}

		// not scanning anymore if we couldn't find a single match
		if !foundNewPid {
			break
		}
	}

	res := make(map[int]*nomadPid)
	for pid := range processFamily {
		np := nomadPid{
			pid:           pid,
			cpuStatsTotal: stats.NewCpuStats(),
			cpuStatsUser:  stats.NewCpuStats(),
			cpuStatsSys:   stats.NewCpuStats(),
		}
		res[pid] = &np
	}
	return res, nil
}

// aggregatedResourceUsage aggregates the resource usage of all the pids and
// returns a TaskResourceUsage data point
func (e *UniversalExecutor) aggregatedResourceUsage(pidStats map[string]*cstructs.ResourceUsage) *cstructs.TaskResourceUsage {
	ts := time.Now().UTC().UnixNano()
	var (
		systemModeCPU, userModeCPU, percent float64
		totalRSS, totalSwap                 uint64
	)

	for _, pidStat := range pidStats {
		systemModeCPU += pidStat.CpuStats.SystemMode
		userModeCPU += pidStat.CpuStats.UserMode
		percent += pidStat.CpuStats.Percent

		totalRSS += pidStat.MemoryStats.RSS
		totalSwap += pidStat.MemoryStats.Swap
	}

	totalCPU := &cstructs.CpuStats{
		SystemMode: systemModeCPU,
		UserMode:   userModeCPU,
		Percent:    percent,
		Measured:   ExecutorBasicMeasuredCpuStats,
		TotalTicks: e.systemCpuStats.TicksConsumed(percent),
	}

	totalMemory := &cstructs.MemoryStats{
		RSS:      totalRSS,
		Swap:     totalSwap,
		Measured: ExecutorBasicMeasuredMemStats,
	}

	resourceUsage := cstructs.ResourceUsage{
		MemoryStats: totalMemory,
		CpuStats:    totalCPU,
	}
	return &cstructs.TaskResourceUsage{
		ResourceUsage: &resourceUsage,
		Timestamp:     ts,
		Pids:          pidStats,
	}
}

// Signal sends the passed signal to the task
func (e *UniversalExecutor) Signal(s os.Signal) error {
	if e.cmd.Process == nil {
		return fmt.Errorf("Task not yet run")
	}

	e.logger.Debug("sending signal to PID", "signal", s, "pid", e.cmd.Process.Pid)
	err := e.cmd.Process.Signal(s)
	if err != nil {
		e.logger.Error("sending signal failed", "signal", s, "err", err)
		return err
	}

	return nil
}

func (e *UniversalExecutor) Stats() (*cstructs.TaskResourceUsage, error) {
	pidStats, err := e.pidStats()
	if err != nil {
		return nil, err
	}
	return e.aggregatedResourceUsage(pidStats), nil
}

func (e *UniversalExecutor) getAllPids() (map[int]*nomadPid, error) {
	allProcesses, err := ps.Processes()
	if err != nil {
		return nil, err
	}
	return e.scanPids(os.Getpid(), allProcesses)
}
