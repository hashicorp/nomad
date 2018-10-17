package qemu

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/utils"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"golang.org/x/net/context"

	"strconv"

	"net"

	"github.com/coreos/go-semver/semver"
	"github.com/hashicorp/nomad/client/driver/executor"
)

const (
	// pluginName is the name of the plugin
	pluginName = "qemu"

	// fingerprintPeriod is the interval at which the driver will send fingerprint responses
	fingerprintPeriod = 30 * time.Second

	// The key populated in Node Attributes to indicate presence of the Qemu driver
	qemuDriverAttr        = "driver.qemu"
	qemuDriverVersionAttr = "driver.qemu.version"
	// Represents an ACPI shutdown request to the VM (emulates pressing a physical power button)
	// Reference: https://en.wikibooks.org/wiki/QEMU/Monitor
	qemuGracefulShutdownMsg = "system_powerdown\n"
	qemuMonitorSocketName   = "qemu-monitor.sock"
	// Maximum socket path length prior to qemu 2.10.1
	qemuLegacyMaxMonitorPathLen = 108
)

var (
	reQemuVersion = regexp.MustCompile(`version (\d[\.\d+]+)`)

	// Prior to qemu 2.10.1, monitor socket paths are truncated to 108 bytes.
	// We should consider this if driver.qemu.version is < 2.10.1 and the
	// generated monitor path is too long.

	//
	// Relevant fix is here:
	// https://github.com/qemu/qemu/commit/ad9579aaa16d5b385922d49edac2c96c79bcfb6
	qemuVersionLongSocketPathFix = semver.New("2.10.1")

	// pluginInfo is the response returned for the PluginInfo RPC
	pluginInfo = &base.PluginInfoResponse{
		Type:             base.PluginTypeDriver,
		PluginApiVersion: "0.0.1",
		PluginVersion:    "0.1.0",
		Name:             pluginName,
	}

	// configSpec is the hcl specification returned by the ConfigSchema RPC
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{})

	// taskConfigSpec is the hcl specification for the driver config section of
	// a taskConfig within a job. It is returned in the TaskConfigSchema RPC
	taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"image_path":        hclspec.NewAttr("image_path", "string", true),
		"accelerator":       hclspec.NewAttr("accelerator", "string", false),
		"graceful_shutdown": hclspec.NewAttr("graceful_shutdown", "bool", false),
		"args":              hclspec.NewAttr("args", "list(string)", false),
		"port_map":          hclspec.NewAttr("port_map", "map(number)", false),
	})

	// capabilities is returned by the Capabilities RPC and indicates what
	// optional features this driver supports
	capabilities = &drivers.Capabilities{
		SendSignals: false,
		Exec:        false,
		FSIsolation: cstructs.FSIsolationImage,
	}

	_ drivers.DriverPlugin = (*QemuDriver)(nil)
)

// Config is the client configuration for the driver
type Config struct {
}

// TaskConfig is the driver configuration of a taskConfig within a job
type TaskConfig struct {
	ImagePath        string         `codec:"image_path" cty:"image_path"`
	Accelerator      string         `codec:"accelerator" cty:"accelerator"`
	Args             []string       `codec:"args" cty:"args"`         // extra arguments to qemu executable
	PortMap          map[string]int `codec:"port_map" cty:"port_map"` // A map of host port and the port name defined in the image manifest file
	GracefulShutdown bool           `codec:"graceful_shutdown" cty:"graceful_shutdown"`
}

// QemuTaskState is the state which is encoded in the handle returned in
// StartTask. This information is needed to rebuild the taskConfig state and handler
// during recovery.
type QemuTaskState struct {
	ReattachConfig *utils.ReattachConfig
	TaskConfig     *drivers.TaskConfig
	Pid            int
	StartedAt      time.Time
}

// QemuDriver is a driver for running images via Qemu
type QemuDriver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *eventer.Eventer

	// config is the driver configuration set by the SetConfig RPC
	config *Config

	// tasks is the in memory datastore mapping taskIDs to execDriverHandles
	tasks *taskStore

	// ctx is the context for the driver. It is passed to other subsystems to
	// coordinate shutdown
	ctx context.Context

	// signalShutdown is called when the driver is shutting down and cancels the
	// ctx passed to any subsystems
	signalShutdown context.CancelFunc

	// logger will log to the plugin output which is usually an 'executor.out'
	// file located in the root of the TaskDir
	logger hclog.Logger
}

func NewQemuDriver(logger hclog.Logger) drivers.DriverPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	logger = logger.Named(pluginName)
	return &QemuDriver{
		eventer:        eventer.NewEventer(ctx, logger),
		config:         &Config{},
		tasks:          newTaskStore(),
		ctx:            ctx,
		signalShutdown: cancel,
		logger:         logger,
	}
}

func (d *QemuDriver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

func (d *QemuDriver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

func (d *QemuDriver) SetConfig(data []byte) error {
	// nothing to do, no driver config
	return nil
}

func (d *QemuDriver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

func (d *QemuDriver) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}

func (r *QemuDriver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	go r.handleFingerprint(ctx, ch)
	return ch, nil
}

func (d *QemuDriver) handleFingerprint(ctx context.Context, ch chan *drivers.Fingerprint) {
	ticker := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(fingerprintPeriod)
			ch <- d.buildFingerprint()
		}
	}
}

func (d *QemuDriver) buildFingerprint() *drivers.Fingerprint {
	fingerprint := &drivers.Fingerprint{
		Attributes:        map[string]string{},
		Health:            drivers.HealthStateHealthy,
		HealthDescription: "healthy",
	}

	bin := "qemu-system-x86_64"
	if runtime.GOOS == "windows" {
		// On windows, the "qemu-system-x86_64" command does not respond to the
		// version flag.
		bin = "qemu-img"
	}
	outBytes, err := exec.Command(bin, "--version").Output()
	if err != nil {
		// return no error, as it isn't an error to not find qemu, it just means we
		// can't use it.
		fingerprint.Health = drivers.HealthStateUndetected
		fingerprint.HealthDescription = ""
		return fingerprint
	}
	out := strings.TrimSpace(string(outBytes))

	matches := reQemuVersion.FindStringSubmatch(out)
	if len(matches) != 2 {
		fingerprint.Health = drivers.HealthStateUndetected
		fingerprint.HealthDescription = fmt.Sprintf("failed to parse qemu version from %v", out)
		return fingerprint
	}
	currentQemuVersion := matches[1]
	fingerprint.Attributes[qemuDriverAttr] = "1"
	fingerprint.Attributes[qemuDriverVersionAttr] = currentQemuVersion
	return fingerprint
}

func (d *QemuDriver) RecoverTask(handle *drivers.TaskHandle) error {
	if handle == nil {
		return fmt.Errorf("error: handle cannot be nil")
	}

	var taskState QemuTaskState
	if err := handle.GetDriverState(&taskState); err != nil {
		d.logger.Error("failed to decode taskConfig state from handle", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to decode taskConfig state from handle: %v", err)
	}

	plugRC, err := utils.ReattachConfigToGoPlugin(taskState.ReattachConfig)
	if err != nil {
		d.logger.Error("failed to build ReattachConfig from taskConfig state", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to build ReattachConfig from taskConfig state: %v", err)
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: plugRC,
	}

	execImpl, pluginClient, err := utils.CreateExecutorWithConfig(pluginConfig, os.Stderr)
	if err != nil {
		d.logger.Error("failed to reattach to executor", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to reattach to executor: %v", err)
	}

	h := &qemuTaskHandle{
		exec:         execImpl,
		pid:          taskState.Pid,
		pluginClient: pluginClient,
		taskConfig:   taskState.TaskConfig,
		procState:    drivers.TaskStateRunning,
		startedAt:    taskState.StartedAt,
		exitResult:   &drivers.ExitResult{},
	}

	d.tasks.Set(taskState.TaskConfig.ID, h)

	go h.run()
	return nil
}

func (d *QemuDriver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *cstructs.DriverNetwork, error) {
	if _, ok := d.tasks.Get(cfg.ID); ok {
		return nil, nil, fmt.Errorf("taskConfig with ID '%s' already started", cfg.ID)
	}

	var driverConfig TaskConfig

	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to decode driver config: %v", err)
	}

	handle := drivers.NewTaskHandle(pluginName)
	handle.Config = cfg

	// Get the image source
	vmPath := driverConfig.ImagePath
	if vmPath == "" {
		return nil, nil, fmt.Errorf("image_path must be set")
	}
	vmID := filepath.Base(vmPath)

	// Parse configuration arguments
	// Create the base arguments
	accelerator := "tcg"
	if driverConfig.Accelerator != "" {
		accelerator = driverConfig.Accelerator
	}

	if cfg.Resources.NomadResources.MemoryMB < 128 || cfg.Resources.NomadResources.MemoryMB > 4000000 {
		return nil, nil, fmt.Errorf("Qemu memory assignment out of bounds")
	}
	mem := fmt.Sprintf("%dM", cfg.Resources.NomadResources.MemoryMB)

	absPath, err := GetAbsolutePath("qemu-system-x86_64")
	if err != nil {
		return nil, nil, err
	}

	args := []string{
		absPath,
		"-machine", "type=pc,accel=" + accelerator,
		"-name", vmID,
		"-m", mem,
		"-drive", "file=" + vmPath,
		"-nographic",
	}

	var monitorPath string
	if driverConfig.GracefulShutdown {
		if runtime.GOOS == "windows" {
			return nil, nil, errors.New("QEMU graceful shutdown is unsupported on the Windows platform")
		}
		// This socket will be used to manage the virtual machine (for example,
		// to perform graceful shutdowns)
		taskDir := filepath.Join(cfg.AllocDir, cfg.Name)
		fingerPrint := d.buildFingerprint()
		if fingerPrint.Attributes == nil {
			return nil, nil, fmt.Errorf("unable to get qemu driver version from fingerprinted attributes")
		}
		monitorPath, err = d.getMonitorPath(taskDir, fingerPrint)
		if err != nil {
			d.logger.Debug("could not get qemu monitor path", "error", err)
			return nil, nil, err
		}
		d.logger.Debug("got monitor path", "monitorPath", monitorPath)
		args = append(args, "-monitor", fmt.Sprintf("unix:%s,server,nowait", monitorPath))
	}

	// Add pass through arguments to qemu executable. A user can specify
	// these arguments in driver task configuration. These arguments are
	// passed directly to the qemu driver as command line options.
	// For example, args = [ "-nodefconfig", "-nodefaults" ]
	// This will allow a VM with embedded configuration to boot successfully.
	args = append(args, driverConfig.Args...)

	// Check the Resources required Networks to add port mappings. If no resources
	// are required, we assume the VM is a purely compute job and does not require
	// the outside world to be able to reach it. VMs ran without port mappings can
	// still reach out to the world, but without port mappings it is effectively
	// firewalled
	protocols := []string{"udp", "tcp"}
	if len(cfg.Resources.NomadResources.Networks) > 0 {
		// Loop through the port map and construct the hostfwd string, to map
		// reserved ports to the ports listenting in the VM
		// Ex: hostfwd=tcp::22000-:22,hostfwd=tcp::80-:8080
		var forwarding []string
		taskPorts := cfg.Resources.NomadResources.Networks[0].PortLabels()
		for label, guest := range driverConfig.PortMap {
			host, ok := taskPorts[label]
			if !ok {
				return nil, nil, fmt.Errorf("Unknown port label %q", label)
			}

			for _, p := range protocols {
				forwarding = append(forwarding, fmt.Sprintf("hostfwd=%s::%d-:%d", p, host, guest))
			}
		}

		if len(forwarding) != 0 {
			args = append(args,
				"-netdev",
				fmt.Sprintf("user,id=user.0,%s", strings.Join(forwarding, ",")),
				"-device", "virtio-net,netdev=user.0",
			)
		}
	}

	// If using KVM, add optimization args
	if accelerator == "kvm" {
		if runtime.GOOS == "windows" {
			return nil, nil, errors.New("KVM accelerator is unsupported on the Windows platform")
		}
		args = append(args,
			"-enable-kvm",
			"-cpu", "host",
			// Do we have cores information available to the Driver?
			// "-smp", fmt.Sprintf("%d", cores),
		)
	}
	d.logger.Debug("starting QemuVM command ", "args", strings.Join(args, " "))

	pluginLogFile := filepath.Join(cfg.TaskDir().Dir, fmt.Sprintf("%s-executor.out", cfg.Name))
	executorConfig := &dstructs.ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: "debug",
	}

	// TODO: best way to pass port ranges in from client config
	execImpl, pluginClient, err := utils.CreateExecutor(os.Stderr, hclog.Debug, 14000, 14512, executorConfig)
	if err != nil {
		return nil, nil, err
	}

	execCmd := &executor.ExecCommand{
		Cmd:        args[0],
		Args:       args[1:],
		Env:        cfg.EnvList(),
		User:       cfg.User,
		TaskDir:    cfg.TaskDir().Dir,
		StdoutPath: cfg.StdoutPath,
		StderrPath: cfg.StderrPath,
	}
	ps, err := execImpl.Launch(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, nil, err
	}
	d.logger.Debug("started new QemuVM", "ID", vmID)

	//TODO(preetha) figure out if monitor path is needed
	h := &qemuTaskHandle{
		exec:         execImpl,
		pid:          ps.Pid,
		monitorPath:  monitorPath,
		pluginClient: pluginClient,
		taskConfig:   cfg,
		procState:    drivers.TaskStateRunning,
		startedAt:    time.Now().Round(time.Millisecond),
		logger:       d.logger,
	}

	qemuDriverState := QemuTaskState{
		ReattachConfig: utils.ReattachConfigFromGoPlugin(pluginClient.ReattachConfig()),
		Pid:            ps.Pid,
		TaskConfig:     cfg,
		StartedAt:      h.startedAt,
	}

	if err := handle.SetDriverState(&qemuDriverState); err != nil {
		d.logger.Error("failed to start task, error setting driver state", "error", err)
		execImpl.Shutdown("", 0)
		pluginClient.Kill()
		return nil, nil, fmt.Errorf("failed to set driver state: %v", err)
	}

	d.tasks.Set(cfg.ID, h)
	go h.run()

	var driverNetwork *cstructs.DriverNetwork
	if len(driverConfig.PortMap) == 1 {
		driverNetwork = &cstructs.DriverNetwork{
			PortMap: driverConfig.PortMap,
		}
	}
	return handle, driverNetwork, nil
}

func (d *QemuDriver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	ch := make(chan *drivers.ExitResult)
	go d.handleWait(ctx, handle, ch)

	return ch, nil
}

func (d *QemuDriver) StopTask(taskID string, timeout time.Duration, signal string) error {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	// Attempt a graceful shutdown only if it was configured in the job
	if handle.monitorPath != "" {
		if err := sendQemuShutdown(d.logger, handle.monitorPath, handle.pid); err != nil {
			d.logger.Debug("error sending graceful shutdown ", "pid", handle.pid, "error", err)
		}
	}

	// TODO(preetha) we are calling shutdown on the executor here
	// after attempting a graceful qemu shutdown, qemu process may
	// not be around when we call exec.shutdown
	if err := handle.exec.Shutdown(signal, timeout); err != nil {
		if handle.pluginClient.Exited() {
			return nil
		}
		return fmt.Errorf("executor Shutdown failed: %v", err)
	}

	return nil
}

func (d *QemuDriver) DestroyTask(taskID string, force bool) error {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	if handle.IsRunning() && !force {
		return fmt.Errorf("cannot destroy running task")
	}

	if !handle.pluginClient.Exited() {
		if handle.IsRunning() {
			if err := handle.exec.Shutdown("", 0); err != nil {
				handle.logger.Error("destroying executor failed", "err", err)
			}
		}

		handle.pluginClient.Kill()
	}

	d.tasks.Delete(taskID)
	return nil
}

func (d *QemuDriver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	handle.stateLock.RLock()
	defer handle.stateLock.RUnlock()

	status := &drivers.TaskStatus{
		ID:          handle.taskConfig.ID,
		Name:        handle.taskConfig.Name,
		State:       handle.procState,
		StartedAt:   handle.startedAt,
		CompletedAt: handle.completedAt,
		ExitResult:  handle.exitResult,
		DriverAttributes: map[string]string{
			"pid": strconv.Itoa(handle.pid),
		},
	}

	return status, nil
}

func (d *QemuDriver) TaskStats(taskID string) (*cstructs.TaskResourceUsage, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	return handle.exec.Stats()
}

func (d *QemuDriver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

func (d *QemuDriver) SignalTask(taskID string, signal string) error {
	return fmt.Errorf("Qemu driver can't signal commands")
}

func (d *QemuDriver) ExecTask(taskID string, cmdArgs []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	return nil, fmt.Errorf("Qemu driver can't execute commands")

}

// GetAbsolutePath returns the absolute path of the passed binary by resolving
// it in the path and following symlinks.
func GetAbsolutePath(bin string) (string, error) {
	lp, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path to %q executable: %v", bin, err)
	}

	return filepath.EvalSymlinks(lp)
}

func (d *QemuDriver) handleWait(ctx context.Context, handle *qemuTaskHandle, ch chan *drivers.ExitResult) {
	defer close(ch)
	var result *drivers.ExitResult
	ps, err := handle.exec.Wait()
	if err != nil {
		result = &drivers.ExitResult{
			Err: fmt.Errorf("executor: error waiting on process: %v", err),
		}
	} else {
		result = &drivers.ExitResult{
			ExitCode: ps.ExitCode,
			Signal:   ps.Signal,
		}
	}

	select {
	case <-ctx.Done():
		return
	case <-d.ctx.Done():
		return
	case ch <- result:
	}
}

// getMonitorPath is used to determine whether a qemu monitor socket can be
// safely created and accessed in the task directory by the version of qemu
// present on the host. If it is safe to use, the socket's full path is
// returned along with a nil error. Otherwise, an empty string is returned
// along with a descriptive error.
func (d *QemuDriver) getMonitorPath(dir string, fingerPrint *drivers.Fingerprint) (string, error) {
	var longPathSupport bool
	currentQemuVer := fingerPrint.Attributes[qemuDriverVersionAttr]
	currentQemuSemver := semver.New(currentQemuVer)
	if currentQemuSemver.LessThan(*qemuVersionLongSocketPathFix) {
		longPathSupport = false
		d.logger.Debug("long socket paths are not available in this version of QEMU", "version", currentQemuVer)
	} else {
		longPathSupport = true
		d.logger.Debug("long socket paths available in this version of QEMU", "version", currentQemuVer)
	}
	fullSocketPath := fmt.Sprintf("%s/%s", dir, qemuMonitorSocketName)
	if len(fullSocketPath) > qemuLegacyMaxMonitorPathLen && longPathSupport == false {
		return "", fmt.Errorf("monitor path is too long for this version of qemu")
	}
	return fullSocketPath, nil
}

// sendQemuShutdown attempts to issue an ACPI power-off command via the qemu
// monitor
func sendQemuShutdown(logger hclog.Logger, monitorPath string, userPid int) error {
	if monitorPath == "" {
		return errors.New("monitorPath not set")
	}
	monitorSocket, err := net.Dial("unix", monitorPath)
	if err != nil {
		logger.Warn("could not connect to qemu monitor", "pid", userPid, "monitorPath", monitorPath, "error", err)
		return err
	}
	defer monitorSocket.Close()
	logger.Debug("sending graceful shutdown command to qemu monitor socket %q for user process pid %d", monitorPath, userPid)
	_, err = monitorSocket.Write([]byte(qemuGracefulShutdownMsg))
	if err != nil {
		logger.Warn("failed to send shutdown message", "shutdown message", qemuGracefulShutdownMsg, "monitorPath", monitorPath, "userPid", userPid, "error", err)
	}
	return err
}
