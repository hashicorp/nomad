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
	"github.com/davecgh/go-spew/spew"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/driver/logging"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/stats"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	lconfigs "github.com/opencontainers/runc/libcontainer/configs"
	"github.com/syndtr/gocapability/capability"
)

const (
	defaultCgroupParent = "nomad"
)

var allCaps []string

var (
	// The statistics the executor exposes when using cgroups
	ExecutorCgroupMeasuredMemStats = []string{"RSS", "Cache", "Swap", "Max Usage", "Kernel Usage", "Kernel Max Usage"}
	ExecutorCgroupMeasuredCpuStats = []string{"System Mode", "User Mode", "Throttled Periods", "Throttled Time", "Percent"}
)

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

	container      libcontainer.Container
	userProc       *libcontainer.Process
	userProcExited chan struct{}
	exitState      *ProcessState

	syslogServer *logging.SyslogServer
	syslogChan   chan *logging.SyslogMessage
}

func (l *LibcontainerExecutor) Launch(command *ExecCommand) (*ProcessState, error) {
	// Find the nomad executable to launch the executor process with
	bin, err := discover.NomadExecutable()
	if err != nil {
		return nil, fmt.Errorf("unable to find the nomad binary: %v", err)
	}

	factory, err := libcontainer.New(
		path.Join(command.TaskDir, "../alloc/container"),
		libcontainer.Cgroupfs,
		libcontainer.InitArgs(bin, "libcontainer-shim"),
	)
	if err != nil {
		wrapped := fmt.Errorf("failed to create factory: %v", err)
		return nil, wrapped
	}

	// A container is groups processes under the same isolation enforcement
	container, err := factory.Create(l.id, newLibcontainerConfig(command))
	if err != nil {
		wrapped := fmt.Errorf("failed to create container(%s): %v", l.id, err)
		return nil, wrapped
	}
	l.container = container

	combined := append([]string{command.Cmd}, command.Args...)
	stdout, err := command.Stdout()
	if err != nil {
		return nil, err
	}
	stderr, err := command.Stderr()
	if err != nil {
		return nil, err
	}

	process := &libcontainer.Process{
		Args:   combined,
		Env:    command.Env,
		Stdout: stdout,
		Stderr: stderr,
		Init:   true,
	}
	l.userProc = process

	l.totalCpuStats = stats.NewCpuStats()
	l.userCpuStats = stats.NewCpuStats()
	l.systemCpuStats = stats.NewCpuStats()

	if err := container.Run(process); err != nil {
		container.Destroy()
		return nil, err
	}

	pid, err := process.Pid()
	if err != nil {
		container.Destroy()
		return nil, err
	}

	l.userProcExited = make(chan struct{})
	go l.wait()

	return &ProcessState{
		Pid:             pid,
		ExitCode:        -1,
		IsolationConfig: l.getIsolationConfig(),
		Time:            time.Now(),
	}, nil
}

func (l *LibcontainerExecutor) Wait() (*ProcessState, error) {
	<-l.userProcExited
	return l.exitState, nil
}

func (l *LibcontainerExecutor) wait() {
	defer close(l.userProcExited)

	ic := l.getIsolationConfig()
	ps, err := l.userProc.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			ps = exitErr.ProcessState
		} else {
			l.logger.Error("failed to call wait on user process", "err", err)
			l.exitState = &ProcessState{Pid: 0, ExitCode: 0, IsolationConfig: ic, Time: time.Now()}
			return
		}
	}

	exitCode := 1
	var signal int
	if status, ok := ps.Sys().(syscall.WaitStatus); ok {
		exitCode = status.ExitStatus()
		spew.Dump(status.Signaled())
		if status.Signaled() {
			const exitSignalBase = 128
			signal = int(status.Signal())
			exitCode = exitSignalBase + signal
		}
	}

	l.exitState = &ProcessState{
		Pid:             ps.Pid(),
		ExitCode:        exitCode,
		Signal:          signal,
		IsolationConfig: ic,
		Time:            time.Now(),
	}
}

func (l *LibcontainerExecutor) Destroy() error {
	if l.container == nil {
		return nil
	}
	defer l.container.Destroy()

	status, err := l.container.Status()
	if err != nil {
		return err
	}

	if status != libcontainer.Stopped {
		return l.container.Signal(os.Kill, true)
	}
	return nil
}

func (l *LibcontainerExecutor) Kill() error {
	return l.container.Signal(os.Interrupt, true)
}

func (l *LibcontainerExecutor) UpdateResources(resources *Resources) error {
	return nil
}

func (l *LibcontainerExecutor) Version() (*ExecutorVersion, error) {
	return &ExecutorVersion{Version: "1.1.0"}, nil
}

func (l *LibcontainerExecutor) Stats() (*cstructs.TaskResourceUsage, error) {
	lstats, err := l.container.Stats()
	if err != nil {
		return nil, err
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
		// TODO Pids
	}
	return &taskResUsage, nil
}

func (l *LibcontainerExecutor) Signal(s os.Signal) error {
	return l.userProc.Signal(s)
}

func (l *LibcontainerExecutor) Exec(deadline time.Time, cmd string, args []string) ([]byte, int, error) {
	combined := append([]string{cmd}, args...)
	// Capture output
	buf, _ := circbuf.NewBuffer(int64(dstructs.CheckBufSize))

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

	stateCh := make(chan *os.ProcessState)
	defer close(stateCh)
	go func() {
		s, err := process.Wait()
		if err == nil {
			stateCh <- s
		}
	}()

	select {
	case ps := <-stateCh:
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

func (l *LibcontainerExecutor) getIsolationConfig() *dstructs.IsolationConfig {
	cfg := &dstructs.IsolationConfig{
		Cgroup:      l.container.Config().Cgroups,
		CgroupPaths: map[string]string{},
	}
	state, err := l.container.State()
	if err != nil {
		return cfg
	}

	cfg.CgroupPaths = state.CgroupPaths
	return cfg
}

func configureCapabilities(cfg *lconfigs.Config, command *ExecCommand) {
	// TODO: allow better control of these
	cfg.Capabilities = &lconfigs.Capabilities{
		Bounding:    allCaps,
		Permitted:   allCaps,
		Inheritable: allCaps,
		Ambient:     allCaps,
		Effective:   allCaps,
	}

}

func configureIsolation(cfg *lconfigs.Config, command *ExecCommand) {
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV

	if command.FSIsolation {
		cfg.Rootfs = command.TaskDir
		cfg.Namespaces = lconfigs.Namespaces{
			{Type: lconfigs.NEWNS},
		}
		cfg.MaskPaths = []string{
			"/proc/kcore",
			"/sys/firmware",
		}

		cfg.ReadonlyPaths = []string{
			"/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus",
		}

		cfg.Devices = lconfigs.DefaultAutoCreatedDevices
		cfg.Mounts = []*lconfigs.Mount{
			/*{
				Source:      "proc",
				Destination: "/proc",
				Device:      "proc",
				Flags:       defaultMountFlags,
			},*/
			{
				Source:      "tmpfs",
				Destination: "/dev",
				Device:      "tmpfs",
				Flags:       syscall.MS_NOSUID | syscall.MS_STRICTATIME,
				Data:        "mode=755",
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
	} else {
		cfg.Rootfs = "/"
		cfg.Cgroups.AllowAllDevices = helper.BoolToPtr(true)
	}
}

func configureCgroups(cfg *lconfigs.Config, command *ExecCommand) error {
	id := uuid.Generate()

	if !command.ResourceLimits {
		// Manually create freezer and devices cgroups
		cfg.Cgroups.Paths = map[string]string{}
		root, err := cgroups.FindCgroupMountpointDir()
		if err != nil {
			return err
		}

		if _, err := os.Stat(root); err != nil {
			return err
		}

		for _, subsystem := range []string{"devices", "freezer"} {
			path, err := cgroups.FindCgroupMountpoint(subsystem)
			if err != nil {
				return fmt.Errorf("failed to find %s cgroup mountpoint: %v", subsystem, err)
			}
			// Sometimes subsystems can be mounted together as 'cpu,cpuacct'.
			path = filepath.Join(root, filepath.Base(path), defaultCgroupParent, id)

			if err = os.MkdirAll(path, 0755); err != nil {
				return err
			}

			cfg.Cgroups.Paths[subsystem] = path
		}

		return nil
	}

	cfg.Cgroups.Path = filepath.Join(defaultCgroupParent, id)
	if command.Resources.MemoryMB > 0 {
		// Total amount of memory allowed to consume
		cfg.Cgroups.Resources.Memory = int64(command.Resources.MemoryMB * 1024 * 1024)
		// Disable swap to avoid issues on the machine
		var memSwappiness uint64 = 0
		cfg.Cgroups.Resources.MemorySwappiness = &memSwappiness
	}

	if command.Resources.CPU < 2 {
		return fmt.Errorf("resources.CPU must be equal to or greater than 2: %v", command.Resources.CPU)
	}

	// Set the relative CPU shares for this cgroup.
	cfg.Cgroups.Resources.CpuShares = uint64(command.Resources.CPU)

	if command.Resources.IOPS != 0 {
		// Validate it is in an acceptable range.
		if command.Resources.IOPS < 10 || command.Resources.IOPS > 1000 {
			return fmt.Errorf("resources.IOPS must be between 10 and 1000: %d", command.Resources.IOPS)
		}

		cfg.Cgroups.Resources.BlkioWeight = uint16(command.Resources.IOPS)
	}
	return nil
}

func newLibcontainerConfig(command *ExecCommand) *lconfigs.Config {
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

	configureCapabilities(cfg, command)
	configureIsolation(cfg, command)
	configureCgroups(cfg, command)
	return cfg
}
