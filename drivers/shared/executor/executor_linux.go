// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/armon/circbuf"
	"github.com/hashicorp/consul-template/signals"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/shared/capabilities"
	"github.com/hashicorp/nomad/drivers/shared/executor/procstats"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	runc "github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	ldevices "github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runc/libcontainer/specconv"
	lutils "github.com/opencontainers/runc/libcontainer/utils"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

const (
	// CPU shares limits are defined by the Linux kernel.
	// https://github.com/torvalds/linux/blob/0dd3ee31125508cd67f7e7172247f05b7fd1753a/kernel/sched/sched.h#L409-L418
	MinCPUShares = 2
	MaxCPUShares = 262_144
)

var (
	// ExecutorCgroupV1MeasuredMemStats is the list of memory stats captured by the executor with cgroup-v1
	ExecutorCgroupV1MeasuredMemStats = []string{"RSS", "Cache", "Swap", "Usage", "Max Usage", "Kernel Usage", "Kernel Max Usage"}

	// ExecutorCgroupV2MeasuredMemStats is the list of memory stats captured by the executor with cgroup-v2. cgroup-v2 exposes different memory stats and no longer reports rss or max usage.
	ExecutorCgroupV2MeasuredMemStats = []string{"Cache", "Swap", "Usage"}

	// ExecutorCgroupMeasuredCpuStats is the list of CPU stats captures by the executor
	ExecutorCgroupMeasuredCpuStats = []string{"System Mode", "User Mode", "Throttled Periods", "Throttled Time", "Percent"}
)

// LibcontainerExecutor implements an Executor with the runc/libcontainer api
type LibcontainerExecutor struct {
	id      string
	command *ExecCommand

	logger hclog.Logger

	compute        cpustats.Compute
	totalCpuStats  *cpustats.Tracker
	userCpuStats   *cpustats.Tracker
	systemCpuStats *cpustats.Tracker
	processStats   procstats.ProcessStats

	container      libcontainer.Container
	userProc       *libcontainer.Process
	userProcExited chan interface{}
	exitState      *ProcessState
	sigChan        chan os.Signal
}

func (l *LibcontainerExecutor) catchSignals() {
	l.logger.Trace("waiting for signals")
	defer signal.Stop(l.sigChan)
	defer close(l.sigChan)

	signal.Notify(l.sigChan, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT, syscall.SIGSEGV)
	for {
		signal := <-l.sigChan
		if signal == syscall.SIGTERM || signal == syscall.SIGINT {
			l.Shutdown("SIGINT", 0)
			break
		}

		if l.container != nil {
			l.container.Signal(signal, false)
		}
	}
}

func NewExecutorWithIsolation(logger hclog.Logger, compute cpustats.Compute) Executor {
	sigch := make(chan os.Signal, 4)

	le := &LibcontainerExecutor{
		id:             strings.ReplaceAll(uuid.Generate(), "-", "_"),
		logger:         logger.Named("isolated_executor"),
		compute:        compute,
		totalCpuStats:  cpustats.New(compute),
		userCpuStats:   cpustats.New(compute),
		systemCpuStats: cpustats.New(compute),
		sigChan:        sigch,
	}

	go le.catchSignals()

	le.processStats = procstats.New(compute, le)
	return le
}

func (l *LibcontainerExecutor) ListProcesses() set.Collection[int] {
	return procstats.List(l.command)
}

// cleanOldProcessesInCGroup kills processes that might ended up orphans when the
// executor was unexpectedly killed and nomad can't reconnect to them.
func (l *LibcontainerExecutor) cleanOldProcessesInCGroup(nomadRelativePath string) {
	l.logger.Debug("looking for old processes", "path", nomadRelativePath)

	root := cgroupslib.GetDefaultRoot()
	orphansPIDs, err := cgroups.GetAllPids(filepath.Join(root, nomadRelativePath))
	if err != nil {
		l.logger.Error("unable to get orphaned task PIDs", "error", err)
		return
	}

	for _, pid := range orphansPIDs {
		l.logger.Info("killing orphaned process", "pid", pid)

		// Avoid bringing down the whole node by mistake, very unlikely case,
		// but it's better to be sure.
		if pid == 1 {
			continue
		}

		err := syscall.Kill(pid, syscall.SIGKILL)
		if err != nil {
			l.logger.Error("unable to send signal to process", "pid", pid, "error", err)
		}
	}
}

// Launch creates a new container in libcontainer and starts a new process with it
func (l *LibcontainerExecutor) Launch(command *ExecCommand) (*ProcessState, error) {
	l.logger.Trace("preparing to launch command", "command", command.Cmd, "args", strings.Join(command.Args, " "))

	if command.Resources == nil {
		command.Resources = &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{},
		}
	}

	l.command = command

	// create a new factory which will store the container state in the allocDir
	factory, err := libcontainer.New(
		path.Join(command.TaskDir, "../alloc/container"),
		// note that os.Args[0] refers to the executor shim typically
		// and first args arguments is ignored now due
		// until https://github.com/opencontainers/runc/pull/1888 is merged
		libcontainer.InitArgs(os.Args[0], "libcontainer-shim"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create factory: %v", err)
	}

	// A container groups processes under the same isolation enforcement
	containerCfg, err := l.newLibcontainerConfig(command)
	if err != nil {
		return nil, fmt.Errorf("failed to configure container(%s): %v", l.id, err)
	}

	l.cleanOldProcessesInCGroup(containerCfg.Cgroups.Path)
	container, err := factory.Create(l.id, containerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create container(%s): %v", l.id, err)
	}
	l.container = container

	// Look up the binary path and make it executable
	taskPath, hostPath, err := lookupTaskBin(command)
	if err != nil {
		return nil, err
	}
	if err := makeExecutable(hostPath); err != nil {
		return nil, err
	}

	combined := append([]string{taskPath}, command.Args...)
	stdout, err := command.Stdout()
	if err != nil {
		return nil, err
	}
	stderr, err := command.Stderr()
	if err != nil {
		return nil, err
	}

	l.logger.Debug("launching", "command", command.Cmd, "args", strings.Join(command.Args, " "))

	// the task process will be started by the container
	process := &libcontainer.Process{
		Args:   combined,
		Env:    command.Env,
		Stdout: stdout,
		Stderr: stderr,
		Init:   true,
	}

	if command.User != "" {
		process.User = command.User
	}

	l.userProc = process

	l.totalCpuStats = cpustats.New(l.compute)
	l.userCpuStats = cpustats.New(l.compute)
	l.systemCpuStats = cpustats.New(l.compute)

	// Starts the task
	if err := container.Run(process); err != nil {
		container.Destroy()
		return nil, err
	}

	pid, err := process.Pid()
	if err != nil {
		container.Destroy()
		return nil, err
	}

	// start a goroutine to wait on the process to complete, so Wait calls can
	// be multiplexed
	l.userProcExited = make(chan interface{})
	go l.wait()

	return &ProcessState{
		Pid:      pid,
		ExitCode: -1,
		Time:     time.Now(),
	}, nil
}

// Wait waits until a process has exited and returns it's exitcode and errors
func (l *LibcontainerExecutor) Wait(ctx context.Context) (*ProcessState, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-l.userProcExited:
		return l.exitState, nil
	}
}

func (l *LibcontainerExecutor) wait() {
	defer close(l.userProcExited)

	// Best effort detection of OOMs. It's possible for us to miss OOM notifications in
	// the event that the wait returns before we read from the OOM notification channel
	var oomKilled atomic.Bool
	go func() {
		oomCh, err := l.container.NotifyOOM()
		if err != nil {
			l.logger.Error("failed to get OOM notification channel for container(%s): %v", l.id, err)
			return
		}

		for range oomCh {
			oomKilled.Store(true)
			// We can terminate this goroutine as soon as we've seen the first OOM
			return
		}
	}()

	ps, err := l.userProc.Wait()
	if err != nil {
		// If the process has exited before we called wait an error is returned
		// the process state is embedded in the error
		if exitErr, ok := err.(*exec.ExitError); ok {
			ps = exitErr.ProcessState
		} else {
			l.logger.Error("failed to call wait on user process", "error", err)
			l.exitState = &ProcessState{Pid: 0, ExitCode: 1, Time: time.Now()}
			return
		}
	}

	l.command.Close()

	exitCode := 1
	var signal int
	if status, ok := ps.Sys().(syscall.WaitStatus); ok {
		exitCode = status.ExitStatus()
		if status.Signaled() {
			const exitSignalBase = 128
			signal = int(status.Signal())
			exitCode = exitSignalBase + signal
		}
	}

	l.exitState = &ProcessState{
		Pid:       ps.Pid(),
		ExitCode:  exitCode,
		Signal:    signal,
		OOMKilled: oomKilled.Load(),
		Time:      time.Now(),
	}
}

// Shutdown stops all processes started and cleans up any resources
// created (such as mountpoints, devices, etc).
func (l *LibcontainerExecutor) Shutdown(signal string, grace time.Duration) error {
	if l.container == nil {
		return nil
	}

	status, err := l.container.Status()
	if err != nil {
		return err
	}

	defer l.container.Destroy()

	if status == libcontainer.Stopped {
		return nil
	}

	if grace > 0 {
		if signal == "" {
			signal = "SIGINT"
		}

		sig, ok := signals.SignalLookup[signal]
		if !ok {
			return fmt.Errorf("error unknown signal given for shutdown: %s", signal)
		}

		// Signal initial container processes only during graceful
		// shutdown; hence `false` arg.
		err = l.container.Signal(sig, false)
		if err != nil {
			return err
		}

		select {
		case <-l.userProcExited:
			return nil
		case <-time.After(grace):
			// Force kill all container processes after grace period,
			// hence `true` argument.
			if err := l.container.Signal(os.Kill, true); err != nil {
				return err
			}
		}
	} else {
		err := l.container.Signal(os.Kill, true)
		if err != nil {
			l.logger.Info("no grace fail", "error", err)
			return err
		}
	}

	select {
	case <-l.userProcExited:
		return nil
	case <-time.After(time.Second * 15):
		return fmt.Errorf("process failed to exit after 15 seconds")
	}
}

// UpdateResources updates the resource isolation with new values to be enforced
func (l *LibcontainerExecutor) UpdateResources(resources *drivers.Resources) error {
	return nil
}

// Version returns the api version of the executor
func (l *LibcontainerExecutor) Version() (*ExecutorVersion, error) {
	return &ExecutorVersion{Version: ExecutorVersionLatest}, nil
}

// Stats returns the resource statistics for processes managed by the executor
func (l *LibcontainerExecutor) Stats(ctx context.Context, interval time.Duration) (<-chan *cstructs.TaskResourceUsage, error) {
	ch := make(chan *cstructs.TaskResourceUsage)
	go l.handleStats(ch, ctx, interval)
	return ch, nil

}

func (l *LibcontainerExecutor) handleStats(ch chan *cstructs.TaskResourceUsage, ctx context.Context, interval time.Duration) {
	defer close(ch)
	timer := time.NewTimer(0)

	var measurableMemStats []string
	switch cgroupslib.GetMode() {
	case cgroupslib.CG1:
		measurableMemStats = ExecutorCgroupV1MeasuredMemStats
	case cgroupslib.CG2:
		measurableMemStats = ExecutorCgroupV2MeasuredMemStats
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-timer.C:
			timer.Reset(interval)
		}

		// the moment we collect this round of stats
		ts := time.Now()

		// get actual stats from the container
		lstats, err := l.container.Stats()
		if err != nil {
			l.logger.Warn("error collecting stats", "error", err)
			return
		}
		stats := lstats.CgroupStats

		// get the map of process pids in this container
		pstats := l.processStats.StatProcesses()

		// Memory Related Stats
		swap := stats.MemoryStats.SwapUsage
		maxUsage := stats.MemoryStats.Usage.MaxUsage
		rss := stats.MemoryStats.Stats["rss"]
		cache := stats.MemoryStats.Stats["cache"]
		mapped_file := stats.MemoryStats.Stats["mapped_file"]
		ms := &cstructs.MemoryStats{
			RSS:            rss,
			Cache:          cache,
			Swap:           swap.Usage,
			MappedFile:     mapped_file,
			Usage:          stats.MemoryStats.Usage.Usage,
			MaxUsage:       maxUsage,
			KernelUsage:    stats.MemoryStats.KernelUsage.Usage,
			KernelMaxUsage: stats.MemoryStats.KernelUsage.MaxUsage,
			Measured:       measurableMemStats,
		}

		// CPU Related Stats
		totalProcessCPUUsage := float64(stats.CpuStats.CpuUsage.TotalUsage)
		userModeTime := float64(stats.CpuStats.CpuUsage.UsageInUsermode)
		kernelModeTime := float64(stats.CpuStats.CpuUsage.UsageInKernelmode)

		totalPercent := l.totalCpuStats.Percent(totalProcessCPUUsage)
		cs := &cstructs.CpuStats{
			SystemMode:       l.systemCpuStats.Percent(kernelModeTime),
			UserMode:         l.userCpuStats.Percent(userModeTime),
			Percent:          totalPercent,
			ThrottledPeriods: stats.CpuStats.ThrottlingData.ThrottledPeriods,
			ThrottledTime:    stats.CpuStats.ThrottlingData.ThrottledTime,
			TotalTicks:       l.systemCpuStats.TicksConsumed(totalPercent),
			Measured:         ExecutorCgroupMeasuredCpuStats,
		}
		taskResUsage := cstructs.TaskResourceUsage{
			ResourceUsage: &cstructs.ResourceUsage{
				MemoryStats: ms,
				CpuStats:    cs,
			},
			Timestamp: ts.UTC().UnixNano(),
			Pids:      pstats,
		}

		select {
		case <-ctx.Done():
			return
		case ch <- &taskResUsage:
		}

	}
}

// Signal sends a signal to the process managed by the executor
func (l *LibcontainerExecutor) Signal(s os.Signal) error {
	return l.userProc.Signal(s)
}

// Exec starts an additional process inside the container
func (l *LibcontainerExecutor) Exec(deadline time.Time, cmd string, args []string) ([]byte, int, error) {
	combined := append([]string{cmd}, args...)
	// Capture output
	buf, _ := circbuf.NewBuffer(int64(drivers.CheckBufSize))

	process := &libcontainer.Process{
		Args:   combined,
		Env:    l.command.Env,
		Stdout: buf,
		Stderr: buf,
	}

	err := l.container.Run(process)
	if err != nil {
		return nil, 0, err
	}

	waitCh := make(chan *waitResult)
	defer close(waitCh)
	go l.handleExecWait(waitCh, process)

	select {
	case result := <-waitCh:
		ps := result.ps
		if result.err != nil {
			if exitErr, ok := result.err.(*exec.ExitError); ok {
				ps = exitErr.ProcessState
			} else {
				return nil, 0, result.err
			}
		}
		var exitCode int
		if status, ok := ps.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
		}
		return buf.Bytes(), exitCode, nil

	case <-time.After(time.Until(deadline)):
		process.Signal(os.Kill)
		return nil, 0, context.DeadlineExceeded
	}

}

func (l *LibcontainerExecutor) newTerminalSocket() (pty func() (*os.File, error), tty *os.File, err error) {
	parent, child, err := lutils.NewSockPair("socket")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create terminal: %v", err)
	}

	return func() (*os.File, error) { return lutils.RecvFd(parent) }, child, err

}

func (l *LibcontainerExecutor) ExecStreaming(ctx context.Context, cmd []string, tty bool,
	stream drivers.ExecTaskStream) error {

	// the task process will be started by the container
	process := &libcontainer.Process{
		Args: cmd,
		Env:  l.userProc.Env,
		User: l.userProc.User,
		Init: false,
		Cwd:  "/",
	}

	execHelper := &execHelper{
		logger: l.logger,

		newTerminal: l.newTerminalSocket,
		setTTY: func(tty *os.File) error {
			process.ConsoleSocket = tty
			return nil
		},
		setIO: func(stdin io.Reader, stdout, stderr io.Writer) error {
			process.Stdin = stdin
			process.Stdout = stdout
			process.Stderr = stderr
			return nil
		},

		processStart: func() error { return l.container.Run(process) },
		processWait: func() (*os.ProcessState, error) {
			return process.Wait()
		},
	}

	return execHelper.run(ctx, tty, stream)

}

type waitResult struct {
	ps  *os.ProcessState
	err error
}

func (l *LibcontainerExecutor) handleExecWait(ch chan *waitResult, process *libcontainer.Process) {
	ps, err := process.Wait()
	ch <- &waitResult{ps, err}
}

func configureCapabilities(cfg *runc.Config, command *ExecCommand) {
	switch command.User {
	case "root":
		// when running as root, use the legacy set of system capabilities, so
		// that we do not break existing nomad clusters using this "feature"
		legacyCaps := capabilities.LegacySupported().Slice(true)
		cfg.Capabilities = &runc.Capabilities{
			Bounding:    legacyCaps,
			Permitted:   legacyCaps,
			Effective:   legacyCaps,
			Ambient:     nil,
			Inheritable: nil,
		}
	default:
		// otherwise apply the plugin + task capability configuration
		//
		// The capabilities must be set in the Ambient set as libcontainer
		// performs `execve`` as an unprivileged user.  Ambient also requires
		// that capabilities are Permitted and Inheritable.  Setting Effective
		// is unnecessary, because we only need the capabilities to become
		// effective _after_ execve, not before.
		cfg.Capabilities = &runc.Capabilities{
			Bounding:    command.Capabilities,
			Permitted:   command.Capabilities,
			Inheritable: command.Capabilities,
			Ambient:     command.Capabilities,
		}
	}
}

func configureNamespaces(pidMode, ipcMode string) runc.Namespaces {
	namespaces := runc.Namespaces{{Type: runc.NEWNS}}
	if pidMode == IsolationModePrivate {
		namespaces = append(namespaces, runc.Namespace{Type: runc.NEWPID})
	}
	if ipcMode == IsolationModePrivate {
		namespaces = append(namespaces, runc.Namespace{Type: runc.NEWIPC})
	}
	return namespaces
}

// configureIsolation prepares the isolation primitives of the container.
// The process runs in a container configured with the following:
//
// * the task directory as the chroot
// * dedicated mount points namespace, but shares the PID, User, domain, network namespaces with host
// * small subset of devices (e.g. stdout/stderr/stdin, tty, shm, pts); default to using the same set of devices as Docker
// * some special filesystems: `/proc`, `/sys`.  Some case is given to avoid exec escaping or setting malicious values through them.
func configureIsolation(cfg *runc.Config, command *ExecCommand) error {
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV

	// set the new root directory for the container
	cfg.Rootfs = command.TaskDir

	// disable pivot_root if set in the driver's configuration
	cfg.NoPivotRoot = command.NoPivotRoot

	// set up default namespaces as configured
	cfg.Namespaces = configureNamespaces(command.ModePID, command.ModeIPC)

	if command.NetworkIsolation != nil {
		cfg.Namespaces = append(cfg.Namespaces, runc.Namespace{
			Type: runc.NEWNET,
			Path: command.NetworkIsolation.Path,
		})
	}

	// paths to mask using a bind mount to /dev/null to prevent reading
	cfg.MaskPaths = []string{
		"/proc/kcore",
		"/sys/firmware",
	}

	// paths that should be remounted as readonly inside the container
	cfg.ReadonlyPaths = []string{
		"/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus",
	}

	cfg.Devices = specconv.AllowedDevices
	if len(command.Devices) > 0 {
		devs, err := cmdDevices(command.Devices)
		if err != nil {
			return err
		}
		cfg.Devices = append(cfg.Devices, devs...)
	}

	for _, device := range cfg.Devices {
		cfg.Cgroups.Resources.Devices = append(cfg.Cgroups.Resources.Devices, &device.Rule)
	}

	cfg.Mounts = []*runc.Mount{
		{
			Source:      "tmpfs",
			Destination: "/dev",
			Device:      "tmpfs",
			Flags:       syscall.MS_NOSUID | syscall.MS_STRICTATIME,
			Data:        "mode=755",
		},
		{
			Source:      "proc",
			Destination: "/proc",
			Device:      "proc",
			Flags:       defaultMountFlags,
		},
		{
			Source:      "devpts",
			Destination: "/dev/pts",
			Device:      "devpts",
			Flags:       syscall.MS_NOSUID | syscall.MS_NOEXEC,
			Data:        "newinstance,ptmxmode=0666,mode=0620,gid=5",
		},
		{
			Device:      "tmpfs",
			Source:      "shm",
			Destination: "/dev/shm",
			Data:        "mode=1777,size=65536k",
			Flags:       defaultMountFlags,
		},
		{
			Source:      "mqueue",
			Destination: "/dev/mqueue",
			Device:      "mqueue",
			Flags:       defaultMountFlags,
		},
		{
			Source:      "sysfs",
			Destination: "/sys",
			Device:      "sysfs",
			Flags:       defaultMountFlags | syscall.MS_RDONLY,
		},
	}

	if len(command.Mounts) > 0 {
		cfg.Mounts = append(cfg.Mounts, cmdMounts(command.Mounts)...)
	}

	return nil
}

func (l *LibcontainerExecutor) configureCgroups(cfg *runc.Config, command *ExecCommand) error {
	// note: an alloc TR hook pre-creates the cgroup(s) in both v1 and v2

	if !command.ResourceLimits {
		return nil
	}

	cg := command.StatsCgroup()
	if cg == "" {
		return errors.New("cgroup must be set")
	}

	// // set the libcontainer hook for writing the PID to cgroup.procs file
	// TODO: this can be cg1 only, right?
	// l.configureCgroupHook(cfg, command)

	// set the libcontainer memory limits
	l.configureCgroupMemory(cfg, command)

	// set cgroup v1/v2 specific attributes (cpu, path)
	switch cgroupslib.GetMode() {
	case cgroupslib.CG1:
		return l.configureCG1(cfg, command, cg)
	default:
		return l.configureCG2(cfg, command, cg)
	}
}

func (*LibcontainerExecutor) configureCgroupHook(cfg *runc.Config, command *ExecCommand) {
	cfg.Hooks = runc.Hooks{
		runc.CreateRuntime: runc.HookList{
			newSetCPUSetCgroupHook(command.Resources.LinuxResources.CpusetCgroupPath),
		},
	}
}

func (l *LibcontainerExecutor) configureCgroupMemory(cfg *runc.Config, command *ExecCommand) {
	// Total amount of memory allowed to consume
	res := command.Resources.NomadResources
	memHard, memSoft := res.Memory.MemoryMaxMB, res.Memory.MemoryMB
	if memHard <= 0 {
		memHard = res.Memory.MemoryMB
		memSoft = 0
	}

	cfg.Cgroups.Resources.Memory = memHard * 1024 * 1024
	cfg.Cgroups.Resources.MemoryReservation = memSoft * 1024 * 1024

	// Disable swap if possible, to avoid issues on the machine
	cfg.Cgroups.Resources.MemorySwappiness = cgroupslib.MaybeDisableMemorySwappiness()
}

func (l *LibcontainerExecutor) configureCG1(cfg *runc.Config, command *ExecCommand, cgroup string) error {

	cpuShares := l.clampCpuShares(command.Resources.LinuxResources.CPUShares)
	cpusetPath := command.Resources.LinuxResources.CpusetCgroupPath
	cpuCores := command.Resources.LinuxResources.CpusetCpus

	// Set the v1 parent relative path (i.e. /nomad/<scope>) for the NON-cpuset cgroups
	scope := filepath.Base(cgroup)
	cfg.Cgroups.Path = filepath.Join("/", cgroupslib.NomadCgroupParent, scope)

	// set cpu resources
	cfg.Cgroups.Resources.CpuShares = uint64(cpuShares)

	// we need to manually set the cpuset, because libcontainer will not set
	// it for our special cpuset cgroup
	if err := l.cpusetCG1(cpusetPath, cpuCores); err != nil {
		return fmt.Errorf("failed to set cpuset: %w", err)
	}

	// tell libcontainer to write the pid to our special cpuset cgroup
	l.configureCgroupHook(cfg, command)

	return nil
}

func (l *LibcontainerExecutor) cpusetCG1(cpusetCgroupPath, cores string) error {
	if cores == "" {
		return nil
	}
	ed := cgroupslib.OpenPath(cpusetCgroupPath)
	return ed.Write("cpuset.cpus", cores)
}

func (l *LibcontainerExecutor) configureCG2(cfg *runc.Config, command *ExecCommand, cg string) error {
	cpuShares := l.clampCpuShares(command.Resources.LinuxResources.CPUShares)
	cpuCores := command.Resources.LinuxResources.CpusetCpus

	// Set the v2 specific unified path
	cfg.Cgroups.Resources.CpusetCpus = cpuCores
	partition := cgroupslib.GetPartitionFromCores(cpuCores)

	// sets cpu.weight, which the kernel also translates to cpu.weight.nice
	// despite what the libcontainer docs say, this sets priority not bandwidth
	cpuWeight := cgroups.ConvertCPUSharesToCgroupV2Value(uint64(cpuShares))
	cfg.Cgroups.Resources.CpuWeight = cpuWeight

	// finally set the path of the cgroup in which to run the task
	scope := filepath.Base(cg)
	cfg.Cgroups.Path = filepath.Join("/", cgroupslib.NomadCgroupParent, partition, scope)

	// todo(shoenig): we will also want to set cpu bandwidth (i.e. cpu_hard_limit)
	// hopefully for 1.7
	return nil
}

func (l *LibcontainerExecutor) newLibcontainerConfig(command *ExecCommand) (*runc.Config, error) {
	cfg := &runc.Config{
		ParentDeathSignal: 9,
		Cgroups: &runc.Cgroup{
			Resources: &runc.Resources{
				MemorySwappiness: nil,
			},
		},
		Version: "1.0.0",
	}

	configureCapabilities(cfg, command)

	// children should not inherit Nomad agent oom_score_adj value
	oomScoreAdj := 0
	cfg.OomScoreAdj = &oomScoreAdj

	if err := configureIsolation(cfg, command); err != nil {
		return nil, err
	}

	if err := l.configureCgroups(cfg, command); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (l *LibcontainerExecutor) clampCpuShares(shares int64) int64 {
	if shares < MinCPUShares {
		l.logger.Warn(
			"task CPU is lower than minimum allowed, using minimum value instead",
			"task_cpu", shares, "min", MinCPUShares,
		)
		return MinCPUShares
	}
	if shares > MaxCPUShares {
		l.logger.Warn(
			"task CPU is greater than maximum allowed, using maximum value instead",
			"task_cpu", shares, "max", MaxCPUShares,
		)
		return MaxCPUShares
	}
	return shares
}

// cmdDevices converts a list of driver.DeviceConfigs into excutor.Devices.
func cmdDevices(driverDevices []*drivers.DeviceConfig) ([]*devices.Device, error) {
	if len(driverDevices) == 0 {
		return nil, nil
	}

	r := make([]*devices.Device, len(driverDevices))

	for i, d := range driverDevices {
		ed, err := ldevices.DeviceFromPath(d.HostPath, d.Permissions)
		if err != nil {
			return nil, fmt.Errorf("failed to make device out for %s: %v", d.HostPath, err)
		}
		ed.Path = d.TaskPath
		ed.Allow = true // rules will be used to allow devices via cgroups
		r[i] = ed
	}

	return r, nil
}

var userMountToUnixMount = map[string]int{
	// Empty string maps to `rprivate` for backwards compatibility in restored
	// older tasks, where mount propagation will not be present.
	"":                                       unix.MS_PRIVATE | unix.MS_REC, // rprivate
	structs.VolumeMountPropagationPrivate:    unix.MS_PRIVATE | unix.MS_REC, // rprivate
	structs.VolumeMountPropagationHostToTask: unix.MS_SLAVE | unix.MS_REC,   // rslave
	structs.VolumeMountPropagationBidirectional: unix.MS_SHARED | unix.MS_REC, // rshared
}

// cmdMounts converts a list of driver.MountConfigs into excutor.Mounts.
func cmdMounts(mounts []*drivers.MountConfig) []*runc.Mount {
	if len(mounts) == 0 {
		return nil
	}

	r := make([]*runc.Mount, len(mounts))

	for i, m := range mounts {
		flags := unix.MS_BIND
		if m.Readonly {
			flags |= unix.MS_RDONLY
		}

		r[i] = &runc.Mount{
			Source:           m.HostPath,
			Destination:      m.TaskPath,
			Device:           "bind",
			Flags:            flags,
			PropagationFlags: []int{userMountToUnixMount[m.PropagationMode]},
		}
	}

	return r
}

// lookupTaskBin finds the file `bin`, searching in order:
//   - taskDir/local
//   - taskDir
//   - each mount, in order listed in the jobspec
//   - a PATH-like search of usr/local/bin/, usr/bin/, and bin/ inside the taskDir
//
// Returns an absolute path inside the container that will get passed as arg[0]
// to the launched process, and the absolute path to that binary as seen by the
// host (these will be identical for binaries that don't come from mounts).
//
// See also executor.lookupBin for a version used by non-isolated drivers.
func lookupTaskBin(command *ExecCommand) (string, string, error) {
	taskDir := command.TaskDir
	bin := command.Cmd

	// Check in the local directory
	localDir := filepath.Join(taskDir, allocdir.TaskLocal)
	taskPath, hostPath, err := getPathInTaskDir(command.TaskDir, localDir, bin)
	if err == nil {
		return taskPath, hostPath, nil
	}

	// Check at the root of the task's directory
	taskPath, hostPath, err = getPathInTaskDir(command.TaskDir, command.TaskDir, bin)
	if err == nil {
		return taskPath, hostPath, nil
	}

	// Check in our mounts
	for _, mount := range command.Mounts {
		taskPath, hostPath, err = getPathInMount(mount.HostPath, mount.TaskPath, bin)
		if err == nil {
			return taskPath, hostPath, nil
		}
	}

	// If there's a / in the binary's path, we can't fallback to a PATH search
	if strings.Contains(bin, "/") {
		return "", "", fmt.Errorf("file %s not found under path %s", bin, taskDir)
	}

	// look for a file using a PATH-style lookup inside the directory
	// root. Similar to the stdlib's exec.LookPath except:
	//   - uses a restricted lookup PATH rather than the agent process's PATH env var.
	//   - does not require that the file is already executable (this will be ensured
	//     by the caller)
	//   - does not prevent using relative path as added to exec.LookPath in go1.19
	//     (this gets fixed-up in the caller)

	// This is a fake PATH so that we're not using the agent's PATH
	restrictedPaths := []string{"/usr/local/bin", "/usr/bin", "/bin"}

	for _, dir := range restrictedPaths {
		pathDir := filepath.Join(command.TaskDir, dir)
		taskPath, hostPath, err = getPathInTaskDir(command.TaskDir, pathDir, bin)
		if err == nil {
			return taskPath, hostPath, nil
		}
	}

	return "", "", fmt.Errorf("file %s not found under path", bin)
}

// getPathInTaskDir searches for the binary in the task directory and nested
// search directory. It returns the absolute path rooted inside the container
// and the absolute path on the host.
func getPathInTaskDir(taskDir, searchDir, bin string) (string, string, error) {

	hostPath := filepath.Join(searchDir, bin)
	err := filepathIsRegular(hostPath)
	if err != nil {
		return "", "", err
	}

	// Find the path relative to the task directory
	rel, err := filepath.Rel(taskDir, hostPath)
	if rel == "" || err != nil {
		return "", "", fmt.Errorf(
			"failed to determine relative path base=%q target=%q: %v",
			taskDir, hostPath, err)
	}

	// Turn relative-to-taskdir path into re-rooted absolute path to avoid
	// libcontainer trying to resolve the binary using $PATH.
	// Do *not* use filepath.Join as it will translate ".."s returned by
	// filepath.Rel. Prepending "/" will cause the path to be rooted in the
	// chroot which is the desired behavior.
	return filepath.Clean("/" + rel), hostPath, nil
}

// getPathInMount for the binary in the mount's host path, constructing the path
// considering that the bin path is rooted in the mount's task path and not its
// host path. It returns the absolute path rooted inside the container and the
// absolute path on the host.
func getPathInMount(mountHostPath, mountTaskPath, bin string) (string, string, error) {

	// Find the path relative to the mount point in the task so that we can
	// trim off any shared prefix when we search on the host path
	mountRel, err := filepath.Rel(mountTaskPath, bin)
	if mountRel == "" || err != nil {
		return "", "", fmt.Errorf("path was not relative to the mount task path")
	}

	hostPath := filepath.Join(mountHostPath, mountRel)

	err = filepathIsRegular(hostPath)
	if err != nil {
		return "", "", err
	}

	// Turn relative-to-taskdir path into re-rooted absolute path to avoid
	// libcontainer trying to resolve the binary using $PATH.
	// Do *not* use filepath.Join as it will translate ".."s returned by
	// filepath.Rel. Prepending "/" will cause the path to be rooted in the
	// chroot which is the desired behavior.
	return filepath.Clean("/" + bin), hostPath, nil
}

// filepathIsRegular verifies that a filepath is a regular file (i.e. not a
// directory, socket, device, etc.)
func filepathIsRegular(path string) error {
	f, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !f.Mode().Type().IsRegular() {
		return fmt.Errorf("path was not a regular file")
	}
	return nil
}

func newSetCPUSetCgroupHook(cgroupPath string) runc.Hook {
	return runc.NewFunctionHook(func(state *specs.State) error {
		return cgroups.WriteCgroupProc(cgroupPath, state.Pid)
	})
}
