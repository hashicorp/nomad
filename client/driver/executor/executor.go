package executor

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
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
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/go-ps"
	"github.com/shirou/gopsutil/process"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/driver/logging"
	"github.com/hashicorp/nomad/client/stats"
	shelpers "github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/nomad/structs"

	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

const (
	// pidScanInterval is the interval at which the executor scans the process
	// tree for finding out the pids that the executor and it's child processes
	// have forked
	pidScanInterval = 5 * time.Second
)

var (
	// The statistics the basic executor exposes
	ExecutorBasicMeasuredMemStats = []string{"RSS", "Swap"}
	ExecutorBasicMeasuredCpuStats = []string{"System Mode", "User Mode", "Percent"}
)

// Executor is the interface which allows a driver to launch and supervise
// a process
type Executor interface {
	SetContext(ctx *ExecutorContext) error
	LaunchCmd(command *ExecCommand) (*ProcessState, error)
	LaunchSyslogServer() (*SyslogServerState, error)
	Wait() (*ProcessState, error)
	ShutDown() error
	Exit() error
	UpdateLogConfig(logConfig *structs.LogConfig) error
	UpdateTask(task *structs.Task) error
	Version() (*ExecutorVersion, error)
	Stats() (*cstructs.TaskResourceUsage, error)
	Signal(s os.Signal) error
	Exec(deadline time.Time, cmd string, args []string) ([]byte, int, error)
}

// ExecutorContext holds context to configure the command user
// wants to run and isolate it
type ExecutorContext struct {
	// TaskEnv holds information about the environment of a Task
	TaskEnv *env.TaskEnv

	// Task is the task whose executor is being launched
	Task *structs.Task

	// AllocID is the allocation id to which the task belongs
	AllocID string

	// TaskDir is the host path to the task's root
	TaskDir string

	// LogDir is the host path where logs should be written
	LogDir string

	// Driver is the name of the driver that invoked the executor
	Driver string

	// PortUpperBound is the upper bound of the ports that we can use to start
	// the syslog server
	PortUpperBound uint

	// PortLowerBound is the lower bound of the ports that we can use to start
	// the syslog server
	PortLowerBound uint
}

// ExecCommand holds the user command, args, and other isolation related
// settings.
type ExecCommand struct {
	// Cmd is the command that the user wants to run.
	Cmd string

	// Args is the args of the command that the user wants to run.
	Args []string

	// FSIsolation determines whether the command would be run in a chroot.
	FSIsolation bool

	// User is the user which the executor uses to run the command.
	User string

	// ResourceLimits determines whether resource limits are enforced by the
	// executor.
	ResourceLimits bool
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

// SyslogServerState holds the address and islation information of a launched
// syslog server
type SyslogServerState struct {
	IsolationConfig *dstructs.IsolationConfig
	Addr            string
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
	ctx     *ExecutorContext
	command *ExecCommand

	pids                map[int]*nomadPid
	pidLock             sync.RWMutex
	exitState           *ProcessState
	processExited       chan interface{}
	fsIsolationEnforced bool

	lre         *logging.FileRotator
	lro         *logging.FileRotator
	rotatorLock sync.Mutex

	shutdownCh chan struct{}

	syslogServer *logging.SyslogServer
	syslogChan   chan *logging.SyslogMessage

	resConCtx resourceContainerContext

	totalCpuStats  *stats.CpuStats
	userCpuStats   *stats.CpuStats
	systemCpuStats *stats.CpuStats
	logger         *log.Logger
}

// NewExecutor returns an Executor
func NewExecutor(logger *log.Logger) Executor {
	if err := shelpers.Init(); err != nil {
		logger.Printf("[ERR] executor: unable to initialize stats: %v", err)
	}

	exec := &UniversalExecutor{
		logger:         logger,
		processExited:  make(chan interface{}),
		totalCpuStats:  stats.NewCpuStats(),
		userCpuStats:   stats.NewCpuStats(),
		systemCpuStats: stats.NewCpuStats(),
		pids:           make(map[int]*nomadPid),
	}

	return exec
}

// Version returns the api version of the executor
func (e *UniversalExecutor) Version() (*ExecutorVersion, error) {
	return &ExecutorVersion{Version: "1.1.0"}, nil
}

// SetContext is used to set the executors context and should be the first call
// after launching the executor.
func (e *UniversalExecutor) SetContext(ctx *ExecutorContext) error {
	e.ctx = ctx
	return nil
}

// LaunchCmd launches the main process and returns its state. It also
// configures an applies isolation on certain platforms.
func (e *UniversalExecutor) LaunchCmd(command *ExecCommand) (*ProcessState, error) {
	e.logger.Printf("[DEBUG] executor: launching command %v %v", command.Cmd, strings.Join(command.Args, " "))

	// Ensure the context has been set first
	if e.ctx == nil {
		return nil, fmt.Errorf("SetContext must be called before launching a command")
	}

	e.command = command

	// setting the user of the process
	if command.User != "" {
		e.logger.Printf("[DEBUG] executor: running command as %s", command.User)
		if err := e.runAs(command.User); err != nil {
			return nil, err
		}
	}

	// set the task dir as the working directory for the command
	e.cmd.Dir = e.ctx.TaskDir

	// configuring the chroot, resource container, and start the plugin
	// process in the chroot.
	if err := e.configureIsolation(); err != nil {
		return nil, err
	}
	// Apply ourselves into the resource container. The executor MUST be in
	// the resource container before the user task is started, otherwise we
	// are subject to a fork attack in which a process escapes isolation by
	// immediately forking.
	if err := e.applyLimits(os.Getpid()); err != nil {
		return nil, err
	}

	// Setup the loggers
	if err := e.configureLoggers(); err != nil {
		return nil, err
	}
	e.cmd.Stdout = e.lro
	e.cmd.Stderr = e.lre

	// Look up the binary path and make it executable
	absPath, err := e.lookupBin(e.ctx.TaskEnv.ReplaceEnv(command.Cmd))
	if err != nil {
		return nil, err
	}

	if err := e.makeExecutable(absPath); err != nil {
		return nil, err
	}

	path := absPath

	// Determine the path to run as it may have to be relative to the chroot.
	if e.fsIsolationEnforced {
		rel, err := filepath.Rel(e.ctx.TaskDir, path)
		if err != nil {
			return nil, fmt.Errorf("failed to determine relative path base=%q target=%q: %v", e.ctx.TaskDir, path, err)
		}
		path = rel
	}

	// Set the commands arguments
	e.cmd.Path = path
	e.cmd.Args = append([]string{e.cmd.Path}, e.ctx.TaskEnv.ParseAndReplace(command.Args)...)
	e.cmd.Env = e.ctx.TaskEnv.List()

	// Start the process
	if err := e.cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command path=%q --- args=%q: %v", path, e.cmd.Args, err)
	}
	go e.collectPids()
	go e.wait()
	ic := e.resConCtx.getIsolationConfig()
	return &ProcessState{Pid: e.cmd.Process.Pid, ExitCode: -1, IsolationConfig: ic, Time: time.Now()}, nil
}

// Exec a command inside a container for exec and java drivers.
func (e *UniversalExecutor) Exec(deadline time.Time, name string, args []string) ([]byte, int, error) {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	return ExecScript(ctx, e.cmd.Dir, e.ctx.TaskEnv, e.cmd.SysProcAttr, name, args)
}

// ExecScript executes cmd with args and returns the output, exit code, and
// error. Output is truncated to client/driver/structs.CheckBufSize
func ExecScript(ctx context.Context, dir string, env *env.TaskEnv, attrs *syscall.SysProcAttr,
	name string, args []string) ([]byte, int, error) {
	name = env.ReplaceEnv(name)
	cmd := exec.CommandContext(ctx, name, env.ParseAndReplace(args)...)

	// Copy runtime environment from the main command
	cmd.SysProcAttr = attrs
	cmd.Dir = dir
	cmd.Env = env.List()

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

// configureLoggers sets up the standard out/error file rotators
func (e *UniversalExecutor) configureLoggers() error {
	e.rotatorLock.Lock()
	defer e.rotatorLock.Unlock()

	logFileSize := int64(e.ctx.Task.LogConfig.MaxFileSizeMB * 1024 * 1024)
	if e.lro == nil {
		lro, err := logging.NewFileRotator(e.ctx.LogDir, fmt.Sprintf("%v.stdout", e.ctx.Task.Name),
			e.ctx.Task.LogConfig.MaxFiles, logFileSize, e.logger)
		if err != nil {
			return fmt.Errorf("error creating new stdout log file for %q: %v", e.ctx.Task.Name, err)
		}
		e.lro = lro
	}

	if e.lre == nil {
		lre, err := logging.NewFileRotator(e.ctx.LogDir, fmt.Sprintf("%v.stderr", e.ctx.Task.Name),
			e.ctx.Task.LogConfig.MaxFiles, logFileSize, e.logger)
		if err != nil {
			return fmt.Errorf("error creating new stderr log file for %q: %v", e.ctx.Task.Name, err)
		}
		e.lre = lre
	}
	return nil
}

// Wait waits until a process has exited and returns it's exitcode and errors
func (e *UniversalExecutor) Wait() (*ProcessState, error) {
	<-e.processExited
	return e.exitState, nil
}

// COMPAT: prior to Nomad 0.3.2, UpdateTask didn't exist.
// UpdateLogConfig updates the log configuration
func (e *UniversalExecutor) UpdateLogConfig(logConfig *structs.LogConfig) error {
	e.ctx.Task.LogConfig = logConfig
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

func (e *UniversalExecutor) UpdateTask(task *structs.Task) error {
	e.ctx.Task = task

	// Updating Log Config
	e.rotatorLock.Lock()
	if e.lro != nil && e.lre != nil {
		fileSize := int64(task.LogConfig.MaxFileSizeMB * 1024 * 1024)
		e.lro.MaxFiles = task.LogConfig.MaxFiles
		e.lro.FileSize = fileSize
		e.lre.MaxFiles = task.LogConfig.MaxFiles
		e.lre.FileSize = fileSize
	}
	e.rotatorLock.Unlock()
	return nil
}

func (e *UniversalExecutor) wait() {
	defer close(e.processExited)
	err := e.cmd.Wait()
	ic := e.resConCtx.getIsolationConfig()
	if err == nil {
		e.exitState = &ProcessState{Pid: 0, ExitCode: 0, IsolationConfig: ic, Time: time.Now()}
		return
	}

	e.lre.Close()
	e.lro.Close()

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
		e.logger.Printf("[DEBUG] executor: unexpected Wait() error type: %v", err)
	}

	e.exitState = &ProcessState{Pid: 0, ExitCode: exitCode, Signal: signal, IsolationConfig: ic, Time: time.Now()}
}

var (
	// finishedErr is the error message received when trying to kill and already
	// exited process.
	finishedErr = "os: process already finished"
)

// ClientCleanup is the cleanup routine that a Nomad Client uses to remove the
// reminants of a child UniversalExecutor.
func ClientCleanup(ic *dstructs.IsolationConfig, pid int) error {
	return clientCleanup(ic, pid)
}

// Exit cleans up the alloc directory, destroys resource container and kills the
// user process
func (e *UniversalExecutor) Exit() error {
	var merr multierror.Error
	if e.syslogServer != nil {
		e.syslogServer.Shutdown()
	}

	if e.lre != nil {
		e.lre.Close()
	}

	if e.lro != nil {
		e.lro.Close()
	}

	// If the executor did not launch a process, return.
	if e.command == nil {
		return nil
	}

	// Prefer killing the process via the resource container.
	if e.cmd.Process != nil && !e.command.ResourceLimits {
		proc, err := os.FindProcess(e.cmd.Process.Pid)
		if err != nil {
			e.logger.Printf("[ERR] executor: can't find process with pid: %v, err: %v",
				e.cmd.Process.Pid, err)
		} else if err := proc.Kill(); err != nil && err.Error() != finishedErr {
			merr.Errors = append(merr.Errors,
				fmt.Errorf("can't kill process with pid: %v, err: %v", e.cmd.Process.Pid, err))
		}
	}

	if e.command.ResourceLimits {
		if err := e.resConCtx.executorCleanup(); err != nil {
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
		return fmt.Errorf("executor.shutdown failed to find process: %v", err)
	}
	if runtime.GOOS == "windows" {
		if err := proc.Kill(); err != nil && err.Error() != finishedErr {
			return err
		}
		return nil
	}
	if err = proc.Signal(os.Interrupt); err != nil && err.Error() != finishedErr {
		return fmt.Errorf("executor.shutdown error: %v", err)
	}
	return nil
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
			e.logger.Printf("[TRACE] executor: unable to create new process with pid: %v", pid)
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
	local := filepath.Join(e.ctx.TaskDir, allocdir.TaskLocal, bin)
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	// Check at the root of the task's directory
	root := filepath.Join(e.ctx.TaskDir, bin)
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

// getFreePort returns a free port ready to be listened on between upper and
// lower bounds
func (e *UniversalExecutor) getListener(lowerBound uint, upperBound uint) (net.Listener, error) {
	if runtime.GOOS == "windows" {
		return e.listenerTCP(lowerBound, upperBound)
	}

	return e.listenerUnix()
}

// listenerTCP creates a TCP listener using an unused port between an upper and
// lower bound
func (e *UniversalExecutor) listenerTCP(lowerBound uint, upperBound uint) (net.Listener, error) {
	for i := lowerBound; i <= upperBound; i++ {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%v", i))
		if err != nil {
			return nil, err
		}
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			continue
		}
		return l, nil
	}
	return nil, fmt.Errorf("No free port found")
}

// listenerUnix creates a Unix domain socket
func (e *UniversalExecutor) listenerUnix() (net.Listener, error) {
	f, err := ioutil.TempFile("", "plugin")
	if err != nil {
		return nil, err
	}
	path := f.Name()

	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := os.Remove(path); err != nil {
		return nil, err
	}

	return net.Listen("unix", path)
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
				e.logger.Printf("[DEBUG] executor: error collecting pids: %v", err)
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

	e.logger.Printf("[DEBUG] executor: sending signal %s to PID %d", s, e.cmd.Process.Pid)
	err := e.cmd.Process.Signal(s)
	if err != nil {
		e.logger.Printf("[ERR] executor: sending signal %v failed: %v", s, err)
		return err
	}

	return nil
}
