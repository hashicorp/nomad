package rawexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hashicorp/consul-template/signals"
	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/utils"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/loader"
	"golang.org/x/net/context"
)

const (
	// pluginName is the name of the plugin
	pluginName = "raw_exec"

	// fingerprintPeriod is the interval at which the driver will send fingerprint responses
	fingerprintPeriod = 30 * time.Second
)

var (
	// PluginID is the rawexec plugin metadata registered in the plugin
	// catalog.
	PluginID = loader.PluginID{
		Name:       pluginName,
		PluginType: base.PluginTypeDriver,
	}

	// PluginConfig is the rawexec factory function registered in the
	// plugin catalog.
	PluginConfig = &loader.InternalPluginConfig{
		Config:  map[string]interface{}{},
		Factory: func(l hclog.Logger) interface{} { return NewRawExecDriver(l) },
	}
)

func PluginLoader(opts map[string]string) (map[string]interface{}, error) {
	conf := map[string]interface{}{}
	if v, err := strconv.ParseBool(opts["driver.raw_exec.enable"]); err == nil {
		conf["enabled"] = v
	}
	if v, err := strconv.ParseBool(opts["driver.raw_exec.no_cgroups"]); err == nil {
		conf["no_cgroups"] = v
	}
	return conf, nil
}

var (
	// pluginInfo is the response returned for the PluginInfo RPC
	pluginInfo = &base.PluginInfoResponse{
		Type:             base.PluginTypeDriver,
		PluginApiVersion: "0.0.1",
		PluginVersion:    "0.1.0",
		Name:             pluginName,
	}

	// configSpec is the hcl specification returned by the ConfigSchema RPC
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"enabled": hclspec.NewDefault(
			hclspec.NewAttr("enabled", "bool", false),
			hclspec.NewLiteral("false"),
		),
		"no_cgroups": hclspec.NewDefault(
			hclspec.NewAttr("no_cgroups", "bool", false),
			hclspec.NewLiteral("false"),
		),
	})

	// taskConfigSpec is the hcl specification for the driver config section of
	// a task within a job. It is returned in the TaskConfigSchema RPC
	taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"command": hclspec.NewAttr("command", "string", true),
		"args":    hclspec.NewAttr("args", "list(string)", false),
	})

	// capabilities is returned by the Capabilities RPC and indicates what
	// optional features this driver supports
	capabilities = &drivers.Capabilities{
		SendSignals: true,
		Exec:        true,
		FSIsolation: cstructs.FSIsolationNone,
	}
)

// RawExecDriver is a privileged version of the exec driver. It provides no
// resource isolation and just fork/execs. The Exec driver should be preferred
// and this should only be used when explicitly needed.
type RawExecDriver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *eventer.Eventer

	// config is the driver configuration set by the SetConfig RPC
	config *Config

	// tasks is the in memory datastore mapping taskIDs to rawExecDriverHandles
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

// Config is the driver configuration set by the SetConfig RPC call
type Config struct {
	// NoCgroups tracks whether we should use a cgroup to manage the process
	// tree
	NoCgroups bool `codec:"no_cgroups" cty:"no_cgroups"`

	// Enabled is set to true to enable the raw_exec driver
	Enabled bool `codec:"enabled" cty:"enabled"`
}

// TaskConfig is the driver configuration of a task within a job
type TaskConfig struct {
	Command string   `codec:"command" cty:"command"`
	Args    []string `codec:"args" cty:"args"`
}

// RawExecTaskState is the state which is encoded in the handle returned in
// StartTask. This information is needed to rebuild the task state and handler
// during recovery.
type RawExecTaskState struct {
	ReattachConfig *utils.ReattachConfig
	TaskConfig     *drivers.TaskConfig
	Pid            int
	StartedAt      time.Time
}

// NewRawExecDriver returns a new DriverPlugin implementation
func NewRawExecDriver(logger hclog.Logger) drivers.DriverPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	logger = logger.Named(pluginName)
	return &RawExecDriver{
		eventer:        eventer.NewEventer(ctx, logger),
		config:         &Config{},
		tasks:          newTaskStore(),
		ctx:            ctx,
		signalShutdown: cancel,
		logger:         logger,
	}
}

func (r *RawExecDriver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

func (r *RawExecDriver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

func (r *RawExecDriver) SetConfig(data []byte) error {
	var config Config
	if err := base.MsgPackDecode(data, &config); err != nil {
		return err
	}

	r.config = &config
	return nil
}

func (r *RawExecDriver) Shutdown(ctx context.Context) error {
	r.signalShutdown()
	return nil
}

func (r *RawExecDriver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

func (r *RawExecDriver) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}

func (r *RawExecDriver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	go r.handleFingerprint(ctx, ch)
	return ch, nil
}

func (r *RawExecDriver) handleFingerprint(ctx context.Context, ch chan *drivers.Fingerprint) {
	defer close(ch)
	ticker := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(fingerprintPeriod)
			ch <- r.buildFingerprint()
		}
	}
}

func (r *RawExecDriver) buildFingerprint() *drivers.Fingerprint {
	var health drivers.HealthState
	var desc string
	attrs := map[string]string{}
	if r.config.Enabled {
		health = drivers.HealthStateHealthy
		desc = "raw_exec enabled"
		attrs["driver.raw_exec"] = "1"
	} else {
		health = drivers.HealthStateUndetected
		desc = "raw_exec disabled"
	}

	return &drivers.Fingerprint{
		Attributes:        map[string]string{},
		Health:            health,
		HealthDescription: desc,
	}
}

func (r *RawExecDriver) RecoverTask(handle *drivers.TaskHandle) error {
	if handle == nil {
		return fmt.Errorf("error: handle cannot be nil")
	}

	var taskState RawExecTaskState
	if err := handle.GetDriverState(&taskState); err != nil {
		r.logger.Error("failed to decode task state from handle", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to decode task state from handle: %v", err)
	}

	plugRC, err := utils.ReattachConfigToGoPlugin(taskState.ReattachConfig)
	if err != nil {
		r.logger.Error("failed to build ReattachConfig from task state", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to build ReattachConfig from task state: %v", err)
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: plugRC,
	}

	exec, pluginClient, err := utils.CreateExecutorWithConfig(pluginConfig, os.Stderr)
	if err != nil {
		r.logger.Error("failed to reattach to executor", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to reattach to executor: %v", err)
	}

	h := &rawExecTaskHandle{
		exec:         exec,
		pid:          taskState.Pid,
		pluginClient: pluginClient,
		task:         taskState.TaskConfig,
		procState:    drivers.TaskStateRunning,
		startedAt:    taskState.StartedAt,
		exitResult:   &drivers.ExitResult{},
	}

	r.tasks.Set(taskState.TaskConfig.ID, h)

	go h.run()
	return nil
}

func (r *RawExecDriver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *cstructs.DriverNetwork, error) {
	if _, ok := r.tasks.Get(cfg.ID); ok {
		return nil, nil, fmt.Errorf("task with ID %q already started", cfg.ID)
	}

	var driverConfig TaskConfig
	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to decode driver config: %v", err)
	}

	r.logger.Info("starting task", "driver_cfg", hclog.Fmt("%+v", driverConfig))
	handle := drivers.NewTaskHandle(pluginName)
	handle.Config = cfg

	pluginLogFile := filepath.Join(cfg.TaskDir().Dir, "executor.out")
	executorConfig := &dstructs.ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: "debug",
	}

	// TODO: best way to pass port ranges in from client config
	exec, pluginClient, err := utils.CreateExecutor(os.Stderr, hclog.Debug, 14000, 14512, executorConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create executor: %v", err)
	}

	execCmd := &executor.ExecCommand{
		Cmd:                driverConfig.Command,
		Args:               driverConfig.Args,
		Env:                cfg.EnvList(),
		User:               cfg.User,
		BasicProcessCgroup: !r.config.NoCgroups,
		TaskDir:            cfg.TaskDir().Dir,
		StdoutPath:         cfg.StdoutPath,
		StderrPath:         cfg.StderrPath,
	}

	ps, err := exec.Launch(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, nil, fmt.Errorf("failed to launch command with executor: %v", err)
	}

	h := &rawExecTaskHandle{
		exec:         exec,
		pid:          ps.Pid,
		pluginClient: pluginClient,
		task:         cfg,
		procState:    drivers.TaskStateRunning,
		startedAt:    time.Now().Round(time.Millisecond),
		logger:       r.logger,
	}

	driverState := RawExecTaskState{
		ReattachConfig: utils.ReattachConfigFromGoPlugin(pluginClient.ReattachConfig()),
		Pid:            ps.Pid,
		TaskConfig:     cfg,
		StartedAt:      h.startedAt,
	}

	if err := handle.SetDriverState(&driverState); err != nil {
		r.logger.Error("failed to start task, error setting driver state", "error", err)
		exec.Shutdown("", 0)
		pluginClient.Kill()
		return nil, nil, fmt.Errorf("failed to set driver state: %v", err)
	}

	r.tasks.Set(cfg.ID, h)
	go h.run()
	return handle, nil, nil
}

func (r *RawExecDriver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	ch := make(chan *drivers.ExitResult)
	go r.handleWait(ctx, handle, ch)

	return ch, nil
}

func (r *RawExecDriver) handleWait(ctx context.Context, handle *rawExecTaskHandle, ch chan *drivers.ExitResult) {
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
	case <-r.ctx.Done():
		return
	case ch <- result:
	}
}

func (r *RawExecDriver) StopTask(taskID string, timeout time.Duration, signal string) error {
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	if err := handle.exec.Shutdown(signal, timeout); err != nil {
		if handle.pluginClient.Exited() {
			return nil
		}
		return fmt.Errorf("executor Shutdown failed: %v", err)
	}

	return nil
}

func (r *RawExecDriver) DestroyTask(taskID string, force bool) error {
	handle, ok := r.tasks.Get(taskID)
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

	r.tasks.Delete(taskID)
	return nil
}

func (r *RawExecDriver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	handle.stateLock.RLock()
	defer handle.stateLock.RUnlock()

	status := &drivers.TaskStatus{
		ID:          handle.task.ID,
		Name:        handle.task.Name,
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

func (r *RawExecDriver) TaskStats(taskID string) (*cstructs.TaskResourceUsage, error) {
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	return handle.exec.Stats()
}

func (r *RawExecDriver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return r.eventer.TaskEvents(ctx)
}

func (r *RawExecDriver) SignalTask(taskID string, signal string) error {
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	sig := os.Interrupt
	if s, ok := signals.SignalLookup[signal]; ok {
		r.logger.Warn("signal to send to task unknown, using SIGINT", "signal", signal, "task_id", handle.task.ID)
		sig = s
	}
	return handle.exec.Signal(sig)
}

func (r *RawExecDriver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	if len(cmd) == 0 {
		return nil, fmt.Errorf("error cmd must have atleast one value")
	}
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	args := []string{}
	if len(cmd) > 1 {
		args = cmd[1:]
	}

	out, exitCode, err := handle.exec.Exec(time.Now().Add(timeout), cmd[0], args)
	if err != nil {
		return nil, err
	}

	return &drivers.ExecTaskResult{
		Stdout: out,
		ExitResult: &drivers.ExitResult{
			ExitCode: exitCode,
		},
	}, nil
}
