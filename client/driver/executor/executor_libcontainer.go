package executor

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"runtime"
	"syscall"
	"time"

	syslog "github.com/RackSec/srslog"
	"github.com/armon/circbuf"
	"github.com/hashicorp/nomad/client/driver/logging"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/stats"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/opencontainers/runc/libcontainer"
	lconfigs "github.com/opencontainers/runc/libcontainer/configs"
)

const (
	defaultCgroupParent  = "nomad"
	defaultSystemdParent = "system.slice"
)

type LibcontainerExecutor struct {
	id  string
	ctx *ExecutorContext

	logger *log.Logger
	logRotator

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

func (l *LibcontainerExecutor) SetContext(ctx *ExecutorContext) error {
	l.ctx = ctx
	return nil
}

func (l *LibcontainerExecutor) LaunchCmd(command *ExecCommand) (*ProcessState, error) {
	bin, err := discover.NomadExecutable()
	if err != nil {
		return nil, fmt.Errorf("unable to find the nomad binary: %v", err)
	}

	if err = l.configureLoggers(l.ctx.LogConfig, l.logger); err != nil {
		return nil, fmt.Errorf("failed to configure logging: %v", err)
	}

	factory, err := libcontainer.New(path.Join(l.ctx.TaskDir, "../alloc/container"),
		libcontainer.Cgroupfs, libcontainer.InitArgs(bin, "libcontainer-shim"))
	if err != nil {
		wrapped := fmt.Errorf("failed to create factory: %v", err)
		return nil, wrapped
	}

	container, err := factory.Create(l.id, newLibcontainerConfig(l.ctx.TaskDir))
	if err != nil {
		wrapped := fmt.Errorf("failed to create container(%s): %v", l.id, err)
		return nil, wrapped
	}
	l.container = container

	combined := append([]string{command.Cmd}, command.Args...)

	process := &libcontainer.Process{
		Args:   combined,
		Env:    l.ctx.Env,
		Stdout: l.lro.processOutWriter,
		Stderr: l.lre.processOutWriter,
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

func (e *LibcontainerExecutor) LaunchSyslogServer() (*SyslogServerState, error) {
	// Ensure the context has been set first
	if e.ctx == nil {
		return nil, fmt.Errorf("SetContext must be called before launching the Syslog Server")
	}

	e.syslogChan = make(chan *logging.SyslogMessage, 2048)
	l, err := e.getListener(e.ctx.PortLowerBound, e.ctx.PortUpperBound)
	if err != nil {
		return nil, err
	}
	e.logger.Printf("[DEBUG] syslog-server: launching syslog server on addr: %v", l.Addr().String())
	if err := e.configureLoggers(e.ctx.LogConfig, e.logger); err != nil {
		return nil, err
	}

	e.syslogServer = logging.NewSyslogServer(l, e.syslogChan, e.logger)
	go e.syslogServer.Start()
	go e.collectLogs(e.lre.rotatorWriter, e.lro.rotatorWriter)
	syslogAddr := fmt.Sprintf("%s://%s", l.Addr().Network(), l.Addr().String())
	return &SyslogServerState{Addr: syslogAddr}, nil
}

func (e *LibcontainerExecutor) collectLogs(we io.Writer, wo io.Writer) {
	for logParts := range e.syslogChan {
		// If the severity of the log line is err then we write to stderr
		// otherwise all messages go to stdout
		if logParts.Severity == syslog.LOG_ERR {
			we.Write(logParts.Message)
			we.Write([]byte{'\n'})
		} else {
			wo.Write(logParts.Message)
			wo.Write([]byte{'\n'})
		}
	}
}

func (e *LibcontainerExecutor) getListener(lowerBound uint, upperBound uint) (net.Listener, error) {
	if runtime.GOOS == "windows" {
		return e.listenerTCP(lowerBound, upperBound)
	}

	return e.listenerUnix()
}

// listenerTCP creates a TCP listener using an unused port between an upper and
// lower bound
func (e *LibcontainerExecutor) listenerTCP(lowerBound uint, upperBound uint) (net.Listener, error) {
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
func (e *LibcontainerExecutor) listenerUnix() (net.Listener, error) {
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
func (l *LibcontainerExecutor) Wait() (*ProcessState, error) {
	<-l.userProcExited
	return l.exitState, nil
}

func (l *LibcontainerExecutor) wait() {
	defer close(l.userProcExited)
	ic := l.getIsolationConfig()
	ps, err := l.userProc.Wait()
	if err != nil {
		l.exitState = &ProcessState{Pid: 0, ExitCode: 0, IsolationConfig: ic, Time: time.Now()}
		return
	}

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
		Pid:             ps.Pid(),
		ExitCode:        exitCode,
		Signal:          signal,
		IsolationConfig: ic,
		Time:            time.Now(),
	}
}

func (l *LibcontainerExecutor) Exit() error {
	defer l.container.Destroy()
	if l.lre != nil {
		l.lre.Close()
	}

	if l.lro != nil {
		l.lro.Close()
	}

	status, err := l.container.Status()
	if err != nil {
		return err
	}

	if status != libcontainer.Stopped {
		return l.container.Signal(os.Kill, true)
	}
	return nil
}

func (l *LibcontainerExecutor) ShutDown() error {
	return l.container.Signal(os.Interrupt, true)
}

func (l *LibcontainerExecutor) UpdateLogConfig(logConfig *LogConfig) error {
	// noop
	return nil
}

func (l *LibcontainerExecutor) UpdateTask(task *structs.Task) error {

	// Updating Log Config
	l.rotatorLock.Lock()
	defer l.rotatorLock.Unlock()
	if l.lro != nil && l.lre != nil {
		fileSize := int64(task.LogConfig.MaxFileSizeMB * 1024 * 1024)
		l.lro.rotatorWriter.MaxFiles = task.LogConfig.MaxFiles
		l.lro.rotatorWriter.FileSize = fileSize
		l.lre.rotatorWriter.MaxFiles = task.LogConfig.MaxFiles
		l.lre.rotatorWriter.FileSize = fileSize
	}
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
		Env:    l.ctx.Env,
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
	return &dstructs.IsolationConfig{
		Cgroup:      l.container.Config().Cgroups,
		CgroupPaths: l.container.Config().Cgroups.Paths,
	}
}

func newLibcontainerConfig(rootfs string) *lconfigs.Config {
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	cg := &lconfigs.Cgroup{
		Resources: &lconfigs.Resources{
			AllowAllDevices: helper.BoolToPtr(true),
		},
		Name: uuid.Generate(),
		//Path: filepath.Join(defaultCgroupParent, uuid.Generate()),
	}
	return &lconfigs.Config{
		Rootfs: rootfs,
		Capabilities: &lconfigs.Capabilities{
			Bounding: []string{
				"CAP_CHOWN",
				"CAP_DAC_OVERRIDE",
				"CAP_FSETID",
				"CAP_FOWNER",
				"CAP_MKNOD",
				"CAP_NET_RAW",
				"CAP_SETGID",
				"CAP_SETUID",
				"CAP_SETFCAP",
				"CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE",
				"CAP_SYS_CHROOT",
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
			Permitted: []string{
				"CAP_CHOWN",
				"CAP_DAC_OVERRIDE",
				"CAP_FSETID",
				"CAP_FOWNER",
				"CAP_MKNOD",
				"CAP_NET_RAW",
				"CAP_SETGID",
				"CAP_SETUID",
				"CAP_SETFCAP",
				"CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE",
				"CAP_SYS_CHROOT",
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
			Inheritable: []string{
				"CAP_CHOWN",
				"CAP_DAC_OVERRIDE",
				"CAP_FSETID",
				"CAP_FOWNER",
				"CAP_MKNOD",
				"CAP_NET_RAW",
				"CAP_SETGID",
				"CAP_SETUID",
				"CAP_SETFCAP",
				"CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE",
				"CAP_SYS_CHROOT",
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
			Ambient: []string{
				"CAP_CHOWN",
				"CAP_DAC_OVERRIDE",
				"CAP_FSETID",
				"CAP_FOWNER",
				"CAP_MKNOD",
				"CAP_NET_RAW",
				"CAP_SETGID",
				"CAP_SETUID",
				"CAP_SETFCAP",
				"CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE",
				"CAP_SYS_CHROOT",
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
			Effective: []string{
				"CAP_CHOWN",
				"CAP_DAC_OVERRIDE",
				"CAP_FSETID",
				"CAP_FOWNER",
				"CAP_MKNOD",
				"CAP_NET_RAW",
				"CAP_SETGID",
				"CAP_SETUID",
				"CAP_SETFCAP",
				"CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE",
				"CAP_SYS_CHROOT",
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
		},
		Namespaces: lconfigs.Namespaces([]lconfigs.Namespace{
			{Type: lconfigs.NEWNS},
			{Type: lconfigs.NEWUTS},
			{Type: lconfigs.NEWIPC},
			{Type: lconfigs.NEWPID},
			//{Type: lconfigs.NEWUSER},
			{Type: lconfigs.NEWNET},
		}),
		Cgroups: cg,
		MaskPaths: []string{
			"/proc/kcore",
			"/sys/firmware",
		},
		ReadonlyPaths: []string{
			"/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus",
		},
		Devices: lconfigs.DefaultAutoCreatedDevices,
		//Hostname: "testing",
		Mounts: []*lconfigs.Mount{
			{
				Source:      "proc",
				Destination: "/proc",
				Device:      "proc",
				Flags:       defaultMountFlags,
			},
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
		},
		//UidMappings: []lconfigs.IDMap{
		//{
		//ContainerID: 0,
		//HostID:      1000,
		//Size:        65536,
		//},
		//},
		//GidMappings: []lconfigs.IDMap{
		//{
		//ContainerID: 0,
		//HostID:      1000,
		//Size:        65536,
		//},
		//},
		Networks: []*lconfigs.Network{
			{
				Type:    "loopback",
				Address: "127.0.0.1/0",
				Gateway: "localhost",
			},
		},
		//		Rlimits: []lconfigs.Rlimit{
		//			{
		//				Type: syscall.RLIMIT_NOFILE,
		//				Hard: uint64(1025),
		//				Soft: uint64(1025),
		//			},
		//		},
		Version: "1.0.0",
	}
}
