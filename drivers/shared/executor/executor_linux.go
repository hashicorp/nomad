// +build linux

package executor

import (
	"context"
	"fmt"
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
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/stats"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/discover"
	shelpers "github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	lconfigs "github.com/opencontainers/runc/libcontainer/configs"
	ldevices "github.com/opencontainers/runc/libcontainer/devices"
	"github.com/syndtr/gocapability/capability"
	"golang.org/x/sys/unix"
)

const (
	defaultCgroupParent = "nomad"
)

var (
	// ExecutorCgroupMeasuredMemStats is the list of memory stats captured by the executor
	ExecutorCgroupMeasuredMemStats = []string{"RSS", "Cache", "Swap", "Usage", "Max Usage", "Kernel Usage", "Kernel Max Usage"}

	// ExecutorCgroupMeasuredCpuStats is the list of CPU stats captures by the executor
	ExecutorCgroupMeasuredCpuStats = []string{"System Mode", "User Mode", "Throttled Periods", "Throttled Time", "Percent"}

	// allCaps is all linux capabilities which is used to configure libcontainer
	allCaps []string
)

// initialize the allCaps var with all capabilities available on the system
func init() {
	last := capability.CAP_LAST_CAP
	// workaround for RHEL6 which has no /proc/sys/kernel/cap_last_cap
	if last == capability.Cap(63) {
		last = capability.CAP_BLOCK_SUSPEND
	}
	for _, cap := range capability.List() {
		if cap > last {
			continue
		}
		allCaps = append(allCaps, fmt.Sprintf("CAP_%s", strings.ToUpper(cap.String())))
	}
}

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
		id:             strings.Replace(uuid.Generate(), "-", "_", 0),
		logger:         logger,
		totalCpuStats:  stats.NewCpuStats(),
		userCpuStats:   stats.NewCpuStats(),
		systemCpuStats: stats.NewCpuStats(),
		pidCollector:   newPidCollector(logger),
	}
}

// Launch creates a new container in libcontainer and starts a new process with it
func (l *LibcontainerExecutor) Launch(command *ExecCommand) (*ProcessState, error) {
	l.logger.Debug("launching command", "command", command.Cmd, "args", strings.Join(command.Args, " "))
	// Find the nomad executable to launch the executor process with
	bin, err := discover.NomadExecutable()
	if err != nil {
		return nil, fmt.Errorf("unable to find the nomad binary: %v", err)
	}

	if command.Resources == nil {
		command.Resources = &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{},
		}
	}

	l.command = command

	// Move to the root cgroup until process is started
	subsystems, err := cgroups.GetAllSubsystems()
	if err != nil {
		return nil, err
	}
	if err := JoinRootCgroup(subsystems); err != nil {
		return nil, err
	}

	// create a new factory which will store the container state in the allocDir
	factory, err := libcontainer.New(
		path.Join(command.TaskDir, "../alloc/container"),
		libcontainer.Cgroupfs,
		libcontainer.InitArgs(bin, "libcontainer-shim"),
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
	absPath, err := lookupBin(command.TaskDir, command.Cmd)
	if err != nil {
		return nil, err
	}

	if err := makeExecutable(absPath); err != nil {
		return nil, err
	}

	path := absPath

	// Determine the path to run as it may have to be relative to the chroot.
	rel, err := filepath.Rel(command.TaskDir, path)
	if err != nil {
		return nil, fmt.Errorf("failed to determine relative path base=%q target=%q: %v", command.TaskDir, path, err)
	}
	path = rel

	combined := append([]string{path}, command.Args...)
	stdout, err := command.Stdout()
	if err != nil {
		return nil, err
	}
	stderr, err := command.Stderr()
	if err != nil {
		return nil, err
	}

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

	// Join process cgroups
	containerState, err := container.State()
	if err != nil {
		l.logger.Error("error entering user process cgroups", "executor_pid", os.Getpid(), "error", err)
	}
	if err := cgroups.EnterPid(containerState.CgroupPaths, os.Getpid()); err != nil {
		l.logger.Error("error entering user process cgroups", "executor_pid", os.Getpid(), "error", err)
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

	// move executor to root cgroup
	subsystems, err := cgroups.GetAllSubsystems()
	if err != nil {
		return err
	}
	if err := JoinRootCgroup(subsystems); err != nil {
		return err
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
			return l.container.Signal(os.Kill, true)
		}
	} else {
		return l.container.Signal(os.Kill, true)
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
		ms := &cstructs.MemoryStats{
			RSS:            rss,
			Cache:          cache,
			Swap:           swap.Usage,
			Usage:          stats.MemoryStats.Usage.Usage,
			MaxUsage:       maxUsage,
			KernelUsage:    stats.MemoryStats.KernelUsage.Usage,
			KernelMaxUsage: stats.MemoryStats.KernelUsage.MaxUsage,
			Measured:       ExecutorCgroupMeasuredMemStats,
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

type waitResult struct {
	ps  *os.ProcessState
	err error
}

func (l *LibcontainerExecutor) handleExecWait(ch chan *waitResult, process *libcontainer.Process) {
	ps, err := process.Wait()
	ch <- &waitResult{ps, err}
}

func configureCapabilities(cfg *lconfigs.Config, command *ExecCommand) error {
	// TODO: allow better control of these
	cfg.Capabilities = &lconfigs.Capabilities{
		Bounding:    allCaps,
		Permitted:   allCaps,
		Inheritable: allCaps,
		Ambient:     allCaps,
		Effective:   allCaps,
	}

	return nil
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

	// launch with mount namespace
	cfg.Namespaces = lconfigs.Namespaces{
		{Type: lconfigs.NEWNS},
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

	cfg.Devices = lconfigs.DefaultAutoCreatedDevices
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
	cfg.Cgroups.Path = filepath.Join(defaultCgroupParent, id)

	if command.Resources == nil || command.Resources.NomadResources == nil {
		return nil
	}

	if mb := command.Resources.NomadResources.Memory.MemoryMB; mb > 0 {
		// Total amount of memory allowed to consume
		cfg.Cgroups.Resources.Memory = mb * 1024 * 1024
		// Disable swap to avoid issues on the machine
		var memSwappiness uint64
		cfg.Cgroups.Resources.MemorySwappiness = &memSwappiness
	}

	cpuShares := command.Resources.NomadResources.Cpu.CpuShares
	if cpuShares < 2 {
		return fmt.Errorf("resources.Cpu.CpuShares must be equal to or greater than 2: %v", cpuShares)
	}

	// Set the relative CPU shares for this cgroup.
	cfg.Cgroups.Resources.CpuShares = uint64(cpuShares)

	return nil
}

func configureBasicCgroups(cfg *lconfigs.Config) error {
	id := uuid.Generate()

	// Manually create freezer cgroup
	cfg.Cgroups.Paths = map[string]string{}
	root, err := cgroups.FindCgroupMountpointDir()
	if err != nil {
		return err
	}

	if _, err := os.Stat(root); err != nil {
		return err
	}

	freezer := cgroupFs.FreezerGroup{}
	subsystem := freezer.Name()
	path, err := cgroups.FindCgroupMountpoint("", subsystem)
	if err != nil {
		return fmt.Errorf("failed to find %s cgroup mountpoint: %v", subsystem, err)
	}
	// Sometimes subsystems can be mounted together as 'cpu,cpuacct'.
	path = filepath.Join(root, filepath.Base(path), defaultCgroupParent, id)

	if err = os.MkdirAll(path, 0755); err != nil {
		return err
	}

	cfg.Cgroups.Paths[subsystem] = path
	return nil
}

func newLibcontainerConfig(command *ExecCommand) (*lconfigs.Config, error) {
	cfg := &lconfigs.Config{
		Cgroups: &lconfigs.Cgroup{
			Resources: &lconfigs.Resources{
				AllowAllDevices:  nil,
				MemorySwappiness: nil,
				AllowedDevices:   lconfigs.DefaultAllowedDevices,
			},
		},
		Version: "1.0.0",
	}

	if err := configureCapabilities(cfg, command); err != nil {
		return nil, err
	}
	if err := configureIsolation(cfg, command); err != nil {
		return nil, err
	}
	if err := configureCgroups(cfg, command); err != nil {
		return nil, err
	}
	return cfg, nil
}

// JoinRootCgroup moves the current process to the cgroups of the init process
func JoinRootCgroup(subsystems []string) error {
	mErrs := new(multierror.Error)
	paths := map[string]string{}
	for _, s := range subsystems {
		mnt, _, err := cgroups.FindCgroupMountpointAndRoot("", s)
		if err != nil {
			multierror.Append(mErrs, fmt.Errorf("error getting cgroup path for subsystem: %s", s))
			continue
		}

		paths[s] = mnt
	}

	err := cgroups.EnterPid(paths, os.Getpid())
	if err != nil {
		multierror.Append(mErrs, err)
	}

	return mErrs.ErrorOrNil()
}

// cmdDevices converts a list of driver.DeviceConfigs into excutor.Devices.
func cmdDevices(devices []*drivers.DeviceConfig) ([]*lconfigs.Device, error) {
	if len(devices) == 0 {
		return nil, nil
	}

	r := make([]*lconfigs.Device, len(devices))

	for i, d := range devices {
		ed, err := ldevices.DeviceFromPath(d.HostPath, d.Permissions)
		if err != nil {
			return nil, fmt.Errorf("failed to make device out for %s: %v", d.HostPath, err)
		}
		ed.Path = d.TaskPath
		r[i] = ed
	}

	return r, nil
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
			Source:      m.HostPath,
			Destination: m.TaskPath,
			Device:      "bind",
			Flags:       flags,
		}
	}

	return r
}
