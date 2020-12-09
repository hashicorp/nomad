package qemu

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/drivers/shared/executor"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

const (
	// pluginName is the name of the plugin
	pluginName = "qemu"

	// fingerprintPeriod is the interval at which the driver will send fingerprint responses
	fingerprintPeriod = 30 * time.Second

	// The key populated in Node Attributes to indicate presence of the Qemu driver
	driverAttr        = "driver.qemu"
	driverVersionAttr = "driver.qemu.version"

	// Represents an ACPI shutdown request to the VM (emulates pressing a physical power button)
	// Reference: https://en.wikibooks.org/wiki/QEMU/Monitor
	qemuGracefulShutdownMsg = "system_powerdown\n"
	qemuMonitorSocketName   = "qemu-monitor.sock"

	// Maximum socket path length prior to qemu 2.10.1
	qemuLegacyMaxMonitorPathLen = 108

	// taskHandleVersion is the version of task handle which this driver sets
	// and understands how to decode driver state
	taskHandleVersion = 1
)

var (
	// PluginID is the qemu plugin metadata registered in the plugin
	// catalog.
	PluginID = loader.PluginID{
		Name:       pluginName,
		PluginType: base.PluginTypeDriver,
	}

	// PluginConfig is the qemu driver factory function registered in the
	// plugin catalog.
	PluginConfig = &loader.InternalPluginConfig{
		Config:  map[string]interface{}{},
		Factory: func(ctx context.Context, l hclog.Logger) interface{} { return NewQemuDriver(ctx, l) },
	}

	versionRegex = regexp.MustCompile(`version (\d[\.\d+]+)`)

	// Prior to qemu 2.10.1, monitor socket paths are truncated to 108 bytes.
	// We should consider this if driver.qemu.version is < 2.10.1 and the
	// generated monitor path is too long.
	//
	// Relevant fix is here:
	// https://github.com/qemu/qemu/commit/ad9579aaa16d5b385922d49edac2c96c79bcfb6
	qemuVersionLongSocketPathFix = semver.New("2.10.1")

	// pluginInfo is the response returned for the PluginInfo RPC
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDriver,
		PluginApiVersions: []string{drivers.ApiVersion010},
		PluginVersion:     "0.1.0",
		Name:              pluginName,
	}

	// configSpec is the hcl specification returned by the ConfigSchema RPC
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"image_paths": hclspec.NewAttr("image_paths", "list(string)", false),
	})

	// taskConfigSpec is the hcl specification for the driver config section of
	// a taskConfig within a job. It is returned in the TaskConfigSchema RPC
	taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"image_path":        hclspec.NewAttr("image_path", "string", true),
		"accelerator":       hclspec.NewAttr("accelerator", "string", false),
		"graceful_shutdown": hclspec.NewAttr("graceful_shutdown", "bool", false),
		"args":              hclspec.NewAttr("args", "list(string)", false),
		"port_map":          hclspec.NewAttr("port_map", "list(map(number))", false),
	})

	// capabilities is returned by the Capabilities RPC and indicates what
	// optional features this driver supports
	capabilities = &drivers.Capabilities{
		SendSignals:  false,
		Exec:         false,
		FSIsolation:  drivers.FSIsolationImage,
		MountConfigs: drivers.MountConfigSupportNone,
	}

	_ drivers.DriverPlugin = (*Driver)(nil)
)

// TaskConfig is the driver configuration of a taskConfig within a job
type TaskConfig struct {
	ImagePath        string             `codec:"image_path"`
	Accelerator      string             `codec:"accelerator"`
	Args             []string           `codec:"args"`     // extra arguments to qemu executable
	PortMap          hclutils.MapStrInt `codec:"port_map"` // A map of host port and the port name defined in the image manifest file
	GracefulShutdown bool               `codec:"graceful_shutdown"`
}

// TaskState is the state which is encoded in the handle returned in StartTask.
// This information is needed to rebuild the taskConfig state and handler
// during recovery.
type TaskState struct {
	ReattachConfig *pstructs.ReattachConfig
	TaskConfig     *drivers.TaskConfig
	Pid            int
	StartedAt      time.Time
}

// Config is the driver configuration set by SetConfig RPC call
type Config struct {
	// ImagePaths is an allow-list of paths qemu is allowed to load an image from
	ImagePaths []string `codec:"image_paths"`
}

// Driver is a driver for running images via Qemu
type Driver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *eventer.Eventer

	// config is the driver configuration set by the SetConfig RPC
	config Config

	// tasks is the in memory datastore mapping taskIDs to qemuTaskHandle
	tasks *taskStore

	// ctx is the context for the driver. It is passed to other subsystems to
	// coordinate shutdown
	ctx context.Context

	// nomadConf is the client agent's configuration
	nomadConfig *base.ClientDriverConfig

	// logger will log to the Nomad agent
	logger hclog.Logger
}

func NewQemuDriver(ctx context.Context, logger hclog.Logger) drivers.DriverPlugin {
	logger = logger.Named(pluginName)
	return &Driver{
		eventer: eventer.NewEventer(ctx, logger),
		tasks:   newTaskStore(),
		ctx:     ctx,
		logger:  logger,
	}
}

func (d *Driver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

func (d *Driver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

func (d *Driver) SetConfig(cfg *base.Config) error {
	var config Config
	if len(cfg.PluginConfig) != 0 {
		if err := base.MsgPackDecode(cfg.PluginConfig, &config); err != nil {
			return err
		}
	}

	d.config = config
	if cfg.AgentConfig != nil {
		d.nomadConfig = cfg.AgentConfig.Driver
	}
	return nil
}

func (d *Driver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

func (d *Driver) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}

func (d *Driver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	go d.handleFingerprint(ctx, ch)
	return ch, nil
}

func (d *Driver) handleFingerprint(ctx context.Context, ch chan *drivers.Fingerprint) {
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

func (d *Driver) buildFingerprint() *drivers.Fingerprint {
	fingerprint := &drivers.Fingerprint{
		Attributes:        map[string]*pstructs.Attribute{},
		Health:            drivers.HealthStateHealthy,
		HealthDescription: drivers.DriverHealthy,
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

	matches := versionRegex.FindStringSubmatch(out)
	if len(matches) != 2 {
		fingerprint.Health = drivers.HealthStateUndetected
		fingerprint.HealthDescription = fmt.Sprintf("Failed to parse qemu version from %v", out)
		return fingerprint
	}
	currentQemuVersion := matches[1]
	fingerprint.Attributes[driverAttr] = pstructs.NewBoolAttribute(true)
	fingerprint.Attributes[driverVersionAttr] = pstructs.NewStringAttribute(currentQemuVersion)
	return fingerprint
}

func (d *Driver) RecoverTask(handle *drivers.TaskHandle) error {
	if handle == nil {
		return fmt.Errorf("error: handle cannot be nil")
	}

	// COMPAT(0.10): pre 0.9 upgrade path check
	if handle.Version == 0 {
		return d.recoverPre09Task(handle)
	}

	// If already attached to handle there's nothing to recover.
	if _, ok := d.tasks.Get(handle.Config.ID); ok {
		d.logger.Trace("nothing to recover; task already exists",
			"task_id", handle.Config.ID,
			"task_name", handle.Config.Name,
		)
		return nil
	}

	var taskState TaskState
	if err := handle.GetDriverState(&taskState); err != nil {
		d.logger.Error("failed to decode taskConfig state from handle", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to decode taskConfig state from handle: %v", err)
	}

	plugRC, err := pstructs.ReattachConfigToGoPlugin(taskState.ReattachConfig)
	if err != nil {
		d.logger.Error("failed to build ReattachConfig from taskConfig state", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to build ReattachConfig from taskConfig state: %v", err)
	}

	execImpl, pluginClient, err := executor.ReattachToExecutor(plugRC,
		d.logger.With("task_name", handle.Config.Name, "alloc_id", handle.Config.AllocID))
	if err != nil {
		d.logger.Error("failed to reattach to executor", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to reattach to executor: %v", err)
	}

	h := &taskHandle{
		exec:         execImpl,
		pid:          taskState.Pid,
		pluginClient: pluginClient,
		taskConfig:   taskState.TaskConfig,
		procState:    drivers.TaskStateRunning,
		startedAt:    taskState.StartedAt,
		exitResult:   &drivers.ExitResult{},
		logger:       d.logger,
	}

	d.tasks.Set(taskState.TaskConfig.ID, h)

	go h.run()
	return nil
}

func isAllowedImagePath(allowedPaths []string, allocDir, imagePath string) bool {
	if !filepath.IsAbs(imagePath) {
		imagePath = filepath.Join(allocDir, imagePath)
	}

	isParent := func(parent, path string) bool {
		rel, err := filepath.Rel(parent, path)
		return err == nil && !strings.HasPrefix(rel, "..")
	}

	// check if path is under alloc dir
	if isParent(allocDir, imagePath) {
		return true
	}

	// check allowed paths
	for _, ap := range allowedPaths {
		if isParent(ap, imagePath) {
			return true
		}
	}

	return false
}

func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	if _, ok := d.tasks.Get(cfg.ID); ok {
		return nil, nil, fmt.Errorf("taskConfig with ID '%s' already started", cfg.ID)
	}

	var driverConfig TaskConfig

	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to decode driver config: %v", err)
	}

	// ensure that PortMap variables are populated early on
	cfg.Env = taskenv.SetPortMapEnvs(cfg.Env, driverConfig.PortMap)

	handle := drivers.NewTaskHandle(taskHandleVersion)
	handle.Config = cfg

	// Get the image source
	vmPath := driverConfig.ImagePath
	if vmPath == "" {
		return nil, nil, fmt.Errorf("image_path must be set")
	}
	vmID := filepath.Base(vmPath)

	if !isAllowedImagePath(d.config.ImagePaths, cfg.AllocDir, vmPath) {
		return nil, nil, fmt.Errorf("image_path is not in the allowed paths")
	}

	// Parse configuration arguments
	// Create the base arguments
	accelerator := "tcg"
	if driverConfig.Accelerator != "" {
		accelerator = driverConfig.Accelerator
	}

	mb := cfg.Resources.NomadResources.Memory.MemoryMB
	if mb < 128 || mb > 4000000 {
		return nil, nil, fmt.Errorf("Qemu memory assignment out of bounds")
	}
	mem := fmt.Sprintf("%dM", mb)

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

	var netdevArgs []string
	if cfg.DNS != nil {
		if len(cfg.DNS.Servers) > 0 {
			netdevArgs = append(netdevArgs, "dns="+cfg.DNS.Servers[0])
		}

		for _, s := range cfg.DNS.Searches {
			netdevArgs = append(netdevArgs, "dnssearch="+s)
		}
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
		taskPorts := cfg.Resources.NomadResources.Networks[0].PortLabels()
		for label, guest := range driverConfig.PortMap {
			host, ok := taskPorts[label]
			if !ok {
				return nil, nil, fmt.Errorf("Unknown port label %q", label)
			}

			for _, p := range protocols {
				netdevArgs = append(netdevArgs, fmt.Sprintf("hostfwd=%s::%d-:%d", p, host, guest))
			}
		}

		if len(netdevArgs) != 0 {
			args = append(args,
				"-netdev",
				fmt.Sprintf("user,id=user.0,%s", strings.Join(netdevArgs, ",")),
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
	executorConfig := &executor.ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: "debug",
	}

	execImpl, pluginClient, err := executor.CreateExecutor(
		d.logger.With("task_name", handle.Config.Name, "alloc_id", handle.Config.AllocID),
		d.nomadConfig, executorConfig)
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

	h := &taskHandle{
		exec:         execImpl,
		pid:          ps.Pid,
		monitorPath:  monitorPath,
		pluginClient: pluginClient,
		taskConfig:   cfg,
		procState:    drivers.TaskStateRunning,
		startedAt:    time.Now().Round(time.Millisecond),
		logger:       d.logger,
	}

	qemuDriverState := TaskState{
		ReattachConfig: pstructs.ReattachConfigFromGoPlugin(pluginClient.ReattachConfig()),
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

	var driverNetwork *drivers.DriverNetwork
	if len(driverConfig.PortMap) == 1 {
		driverNetwork = &drivers.DriverNetwork{
			PortMap: driverConfig.PortMap,
		}
	}
	return handle, driverNetwork, nil
}

func (d *Driver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	ch := make(chan *drivers.ExitResult)
	go d.handleWait(ctx, handle, ch)

	return ch, nil
}

func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
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

func (d *Driver) DestroyTask(taskID string, force bool) error {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	if handle.IsRunning() && !force {
		return fmt.Errorf("cannot destroy running task")
	}

	if !handle.pluginClient.Exited() {
		if err := handle.exec.Shutdown("", 0); err != nil {
			handle.logger.Error("destroying executor failed", "err", err)
		}

		handle.pluginClient.Kill()
	}

	d.tasks.Delete(taskID)
	return nil
}

func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	return handle.TaskStatus(), nil
}

func (d *Driver) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	return handle.exec.Stats(ctx, interval)
}

func (d *Driver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

func (d *Driver) SignalTask(taskID string, signal string) error {
	return fmt.Errorf("Qemu driver can't signal commands")
}

func (d *Driver) ExecTask(taskID string, cmdArgs []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
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

func (d *Driver) handleWait(ctx context.Context, handle *taskHandle, ch chan *drivers.ExitResult) {
	defer close(ch)
	var result *drivers.ExitResult
	ps, err := handle.exec.Wait(ctx)
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
	case <-d.ctx.Done():
	case ch <- result:
	}
}

// getMonitorPath is used to determine whether a qemu monitor socket can be
// safely created and accessed in the task directory by the version of qemu
// present on the host. If it is safe to use, the socket's full path is
// returned along with a nil error. Otherwise, an empty string is returned
// along with a descriptive error.
func (d *Driver) getMonitorPath(dir string, fingerPrint *drivers.Fingerprint) (string, error) {
	var longPathSupport bool
	currentQemuVer := fingerPrint.Attributes[driverVersionAttr]
	currentQemuSemver := semver.New(currentQemuVer.GoString())
	if currentQemuSemver.LessThan(*qemuVersionLongSocketPathFix) {
		longPathSupport = false
		d.logger.Debug("long socket paths are not available in this version of QEMU", "version", currentQemuVer)
	} else {
		longPathSupport = true
		d.logger.Debug("long socket paths available in this version of QEMU", "version", currentQemuVer)
	}
	fullSocketPath := fmt.Sprintf("%s/%s", dir, qemuMonitorSocketName)
	if len(fullSocketPath) > qemuLegacyMaxMonitorPathLen && !longPathSupport {
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
	logger.Debug("sending graceful shutdown command to qemu monitor socket", "monitor_path", monitorPath, "pid", userPid)
	_, err = monitorSocket.Write([]byte(qemuGracefulShutdownMsg))
	if err != nil {
		logger.Warn("failed to send shutdown message", "shutdown message", qemuGracefulShutdownMsg, "monitorPath", monitorPath, "userPid", userPid, "error", err)
	}
	return err
}
