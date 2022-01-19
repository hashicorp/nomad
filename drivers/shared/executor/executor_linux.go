//go:build linux
// +build linux

package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/armon/circbuf"
	"github.com/hashicorp/consul-template/signals"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/stats"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/shared/capabilities"
	shelpers "github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	lconfigs "github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	ldevices "github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runc/libcontainer/specconv"
	lutils "github.com/opencontainers/runc/libcontainer/utils"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

const (
	defaultCgroupParent = "/nomad"
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

	totalCpuStats  *stats.CpuStats
	userCpuStats   *stats.CpuStats
	systemCpuStats *stats.CpuStats
	pidCollector   *pidCollector

	container      libcontainer.Container
	userProc       *libcontainer.Process
	userProcExited chan interface{}
	exitState      *ProcessState
}

func NewExecutorWithIsolation(logger hclog.Logger) Executor {
	logger = logger.Named("isolated_executor")
	if err := shelpers.Init(); err != nil {
		logger.Error("unable to initialize stats", "error", err)
	}
	return &LibcontainerExecutor{
		id:             strings.ReplaceAll(uuid.Generate(), "-", "_"),
		logger:         logger,
		totalCpuStats:  stats.NewCpuStats(),
		userCpuStats:   stats.NewCpuStats(),
		systemCpuStats: stats.NewCpuStats(),
		pidCollector:   newPidCollector(logger),
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
		libcontainer.Cgroupfs,
		// note that os.Args[0] refers to the executor shim typically
		// and first args arguments is ignored now due
		// until https://github.com/opencontainers/runc/pull/1888 is merged
		libcontainer.InitArgs(os.Args[0], "libcontainer-shim"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create factory: %v", err)
	}

	// A container groups processes under the same isolation enforcement
	containerCfg, err := newLibcontainerConfig(command)
	if err != nil {
		return nil, fmt.Errorf("failed to configure container(%s): %v", l.id, err)
	}

	container, err := factory.Create(l.id, containerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create container(%s): %v", l.id, err)
	}
	l.container = container

	// Look up the binary path and make it executable
	absPath, err := lookupTaskBin(command)

	if err != nil {
		return nil, err
	}

	if err := makeExecutable(absPath); err != nil {
		return nil, err
	}

	path := absPath

	// Ensure that the path is contained in the chroot, and find it relative to the container
	rel, err := filepath.Rel(command.TaskDir, path)
	if err != nil {
		return nil, fmt.Errorf("failed to determine relative path base=%q target=%q: %v", command.TaskDir, path, err)
	}

	// Turn relative-to-chroot path into absolute path to avoid
	// libcontainer trying to resolve the binary using $PATH.
	// Do *not* use filepath.Join as it will translate ".."s returned by
	// filepath.Rel. Prepending "/" will cause the path to be rooted in the
	// chroot which is the desired behavior.
	path = "/" + rel

	combined := append([]string{path}, command.Args...)
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

	l.totalCpuStats = stats.NewCpuStats()
	l.userCpuStats = stats.NewCpuStats()
	l.systemCpuStats = stats.NewCpuStats()

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
	go l.pidCollector.collectPids(l.userProcExited, l.getAllPids)
	go l.wait()

	return &ProcessState{
		Pid:      pid,
		ExitCode: -1,
		Time:     time.Now(),
	}, nil
}

func (l *LibcontainerExecutor) getAllPids() (map[int]*nomadPid, error) {
	pids, err := l.container.Processes()
	if err != nil {
		return nil, err
	}
	nPids := make(map[int]*nomadPid)
	for _, pid := range pids {
		nPids[pid] = &nomadPid{
			pid:           pid,
			cpuStatsTotal: stats.NewCpuStats(),
			cpuStatsUser:  stats.NewCpuStats(),
			cpuStatsSys:   stats.NewCpuStats(),
		}
	}
	return nPids, nil
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
		Pid:      ps.Pid(),
		ExitCode: exitCode,
		Signal:   signal,
		Time:     time.Now(),
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

	measuredMemStats := ExecutorCgroupV1MeasuredMemStats
	if cgroups.IsCgroup2UnifiedMode() {
		measuredMemStats = ExecutorCgroupV2MeasuredMemStats
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-timer.C:
			timer.Reset(interval)
		}

		lstats, err := l.container.Stats()
		if err != nil {
			l.logger.Warn("error collecting stats", "error", err)
			return
		}

		pidStats, err := l.pidCollector.pidStats()
		if err != nil {
			l.logger.Warn("error collecting stats", "error", err)
			return
		}

		ts := time.Now()
		stats := lstats.CgroupStats

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
			Measured:       measuredMemStats,
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
			Pids:      pidStats,
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

func configureCapabilities(cfg *lconfigs.Config, command *ExecCommand) {
	switch command.User {
	case "root":
		// when running as root, use the legacy set of system capabilities, so
		// that we do not break existing nomad clusters using this "feature"
		legacyCaps := capabilities.LegacySupported().Slice(true)
		cfg.Capabilities = &lconfigs.Capabilities{
			Bounding:    legacyCaps,
			Permitted:   legacyCaps,
			Effective:   legacyCaps,
			Ambient:     nil,
			Inheritable: nil,
		}
	default:
		// otherwise apply the plugin + task capability configuration
		cfg.Capabilities = &lconfigs.Capabilities{
			Bounding: command.Capabilities,
		}
	}
}

func configureNamespaces(pidMode, ipcMode string) lconfigs.Namespaces {
	namespaces := lconfigs.Namespaces{{Type: lconfigs.NEWNS}}
	if pidMode == IsolationModePrivate {
		namespaces = append(namespaces, lconfigs.Namespace{Type: lconfigs.NEWPID})
	}
	if ipcMode == IsolationModePrivate {
		namespaces = append(namespaces, lconfigs.Namespace{Type: lconfigs.NEWIPC})
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
func configureIsolation(cfg *lconfigs.Config, command *ExecCommand) error {
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV

	// set the new root directory for the container
	cfg.Rootfs = command.TaskDir

	// disable pivot_root if set in the driver's configuration
	cfg.NoPivotRoot = command.NoPivotRoot

	// set up default namespaces as configured
	cfg.Namespaces = configureNamespaces(command.ModePID, command.ModeIPC)

	if command.NetworkIsolation != nil {
		cfg.Namespaces = append(cfg.Namespaces, lconfigs.Namespace{
			Type: lconfigs.NEWNET,
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

	cfg.Mounts = []*lconfigs.Mount{
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

func configureCgroups(cfg *lconfigs.Config, command *ExecCommand) error {

	// If resources are not limited then manually create cgroups needed
	if !command.ResourceLimits {
		return configureBasicCgroups(cfg)
	}

	id := uuid.Generate()
	cfg.Cgroups.Path = filepath.Join("/", defaultCgroupParent, id)

	if command.Resources == nil || command.Resources.NomadResources == nil {
		return nil
	}

	// Total amount of memory allowed to consume
	res := command.Resources.NomadResources
	memHard, memSoft := res.Memory.MemoryMaxMB, res.Memory.MemoryMB
	if memHard <= 0 {
		memHard = res.Memory.MemoryMB
		memSoft = 0
	}

	if memHard > 0 {
		cfg.Cgroups.Resources.Memory = memHard * 1024 * 1024
		cfg.Cgroups.Resources.MemoryReservation = memSoft * 1024 * 1024

		// Disable swap to avoid issues on the machine
		var memSwappiness uint64
		cfg.Cgroups.Resources.MemorySwappiness = &memSwappiness
	}

	cpuShares := res.Cpu.CpuShares
	if cpuShares < 2 {
		return fmt.Errorf("resources.Cpu.CpuShares must be equal to or greater than 2: %v", cpuShares)
	}

	// Set the relative CPU shares for this cgroup, and convert for cgroupv2
	cfg.Cgroups.Resources.CpuShares = uint64(cpuShares)
	cfg.Cgroups.Resources.CpuWeight = cgroups.ConvertCPUSharesToCgroupV2Value(uint64(cpuShares))

	if command.Resources.LinuxResources != nil && command.Resources.LinuxResources.CpusetCgroupPath != "" {
		cfg.Hooks = lconfigs.Hooks{
			lconfigs.CreateRuntime: lconfigs.HookList{
				newSetCPUSetCgroupHook(command.Resources.LinuxResources.CpusetCgroupPath),
			},
		}
	}

	return nil
}

func configureBasicCgroups(cfg *lconfigs.Config) error {
	id := uuid.Generate()

	// Manually create freezer cgroup

	subsystem := "freezer"

	path, err := getCgroupPathHelper(subsystem, filepath.Join(defaultCgroupParent, id))
	if err != nil {
		return fmt.Errorf("failed to find %s cgroup mountpoint: %v", subsystem, err)
	}

	if err = os.MkdirAll(path, 0755); err != nil {
		return err
	}

	cfg.Cgroups.Paths = map[string]string{
		subsystem: path,
	}
	return nil
}

func getCgroupPathHelper(subsystem, cgroup string) (string, error) {
	mnt, root, err := cgroups.FindCgroupMountpointAndRoot("", subsystem)
	if err != nil {
		return "", err
	}

	// This is needed for nested containers, because in /proc/self/cgroup we
	// see paths from host, which don't exist in container.
	relCgroup, err := filepath.Rel(root, cgroup)
	if err != nil {
		return "", err
	}

	return filepath.Join(mnt, relCgroup), nil
}

func newLibcontainerConfig(command *ExecCommand) (*lconfigs.Config, error) {
	cfg := &lconfigs.Config{
		Cgroups: &lconfigs.Cgroup{
			Resources: &lconfigs.Resources{
				MemorySwappiness: nil,
			},
		},
		Version: "1.0.0",
	}

	for _, device := range specconv.AllowedDevices {
		cfg.Cgroups.Resources.Devices = append(cfg.Cgroups.Resources.Devices, &device.Rule)
	}

	configureCapabilities(cfg, command)

	// children should not inherit Nomad agent oom_score_adj value
	oomScoreAdj := 0
	cfg.OomScoreAdj = &oomScoreAdj

	if err := configureIsolation(cfg, command); err != nil {
		return nil, err
	}

	if err := configureCgroups(cfg, command); err != nil {
		return nil, err
	}

	return cfg, nil
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
func cmdMounts(mounts []*drivers.MountConfig) []*lconfigs.Mount {
	if len(mounts) == 0 {
		return nil
	}

	r := make([]*lconfigs.Mount, len(mounts))

	for i, m := range mounts {
		flags := unix.MS_BIND
		if m.Readonly {
			flags |= unix.MS_RDONLY
		}

		r[i] = &lconfigs.Mount{
			Source:           m.HostPath,
			Destination:      m.TaskPath,
			Device:           "bind",
			Flags:            flags,
			PropagationFlags: []int{userMountToUnixMount[m.PropagationMode]},
		}
	}

	return r
}

// lookupTaskBin finds the file `bin` in taskDir/local, taskDir in that order, then performs
// a PATH search inside taskDir. It returns an absolute path. See also executor.lookupBin
func lookupTaskBin(command *ExecCommand) (string, error) {
	taskDir := command.TaskDir
	bin := command.Cmd

	// Check in the local directory
	localDir := filepath.Join(taskDir, allocdir.TaskLocal)
	local := filepath.Join(localDir, bin)
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	// Check at the root of the task's directory
	root := filepath.Join(taskDir, bin)
	if _, err := os.Stat(root); err == nil {
		return root, nil
	}

	if strings.Contains(bin, "/") {
		return "", fmt.Errorf("file %s not found under path %s", bin, taskDir)
	}

	path := "/usr/local/bin:/usr/bin:/bin"

	return lookPathIn(path, taskDir, bin)
}

// lookPathIn looks for a file with PATH inside the directory root. Like exec.LookPath
func lookPathIn(path string, root string, bin string) (string, error) {
	// exec.LookPath(file string)
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// match unix shell behavior, empty path element == .
			dir = "."
		}
		path := filepath.Join(root, dir, bin)
		f, err := os.Stat(path)
		if err != nil {
			continue
		}
		if m := f.Mode(); !m.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("file %s not found under path %s", bin, root)
}

func newSetCPUSetCgroupHook(cgroupPath string) lconfigs.Hook {
	return lconfigs.NewFunctionHook(func(state *specs.State) error {
		return cgroups.WriteCgroupProc(cgroupPath, state.Pid)
	})
}
