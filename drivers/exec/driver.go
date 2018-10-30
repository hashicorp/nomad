package exec

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
	"golang.org/x/net/context"
)

const (
	// pluginName is the name of the plugin
	pluginName = "exec"

	// fingerprintPeriod is the interval at which the driver will send fingerprint responses
	fingerprintPeriod = 30 * time.Second
)

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
			hclspec.NewLiteral("true"),
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
		FSIsolation: cstructs.FSIsolationChroot,
	}
)

// ExecDriver fork/execs tasks using many of the underlying OS's isolation
// features where configured.
type ExecDriver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *eventer.Eventer

	// config is the driver configuration set by the SetConfig RPC
	config *Config

	// nomadConfig is the client config from nomad
	nomadConfig *base.ClientDriverConfig

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

// Config is the driver configuration set by the SetConfig RPC call
type Config struct {
	// Enabled is set to true to enable the driver
	Enabled bool `cty:"enabled"`
}

// TaskConfig is the driver configuration of a task within a job
type TaskConfig struct {
	Command string   `cty:"command"`
	Args    []string `cty:"args"`
}

// TaskState is the state which is encoded in the handle returned in
// StartTask. This information is needed to rebuild the task state and handler
// during recovery.
type TaskState struct {
	ReattachConfig *utils.ReattachConfig
	TaskConfig     *drivers.TaskConfig
	Pid            int
	StartedAt      time.Time
}

// NewExecDriver returns a new DrivePlugin implementation
func NewExecDriver(logger hclog.Logger) drivers.DriverPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	logger = logger.Named(pluginName)
	return &ExecDriver{
		eventer:        eventer.NewEventer(ctx, logger),
		config:         &Config{},
		tasks:          newTaskStore(),
		ctx:            ctx,
		signalShutdown: cancel,
		logger:         logger,
	}
}

func (*ExecDriver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

func (*ExecDriver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

func (d *ExecDriver) SetConfig(data []byte, cfg *base.ClientAgentConfig) error {
	var config Config
	if err := base.MsgPackDecode(data, &config); err != nil {
		return err
	}

	d.config = &config
	if cfg != nil {
		d.nomadConfig = cfg.Driver
	}
	return nil
}

func (d *ExecDriver) Shutdown(ctx context.Context) error {
	d.signalShutdown()
	return nil
}

func (d *ExecDriver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

func (d *ExecDriver) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}

func (d *ExecDriver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	go d.handleFingerprint(ctx, ch)
	return ch, nil

}
func (d *ExecDriver) handleFingerprint(ctx context.Context, ch chan *drivers.Fingerprint) {
	defer close(ch)
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

func (d *ExecDriver) RecoverTask(handle *drivers.TaskHandle) error {
	var taskState TaskState
	logger := d.logger.With("task_id", handle.Config.ID)

	err := handle.GetDriverState(&taskState)
	if err != nil {
		logger.Error("failed to decode driver state during task recovery", "error", err)
		return fmt.Errorf("failed to decode state: %v", err)
	}

	plugRC, err := utils.ReattachConfigToGoPlugin(taskState.ReattachConfig)
	if err != nil {
		logger.Error("failed to build reattach config during task recovery", "error", err)
		return fmt.Errorf("failed to build reattach config: %v", err)
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: plugRC,
	}

	exec, pluginClient, err := utils.CreateExecutorWithConfig(pluginConfig, os.Stderr)
	if err != nil {
		logger.Error("failed to build executor during task recovery", "error", err)
		return fmt.Errorf("failed to build executor: %v", err)
	}

	h := &execTaskHandle{
		exec:         exec,
		pid:          taskState.Pid,
		pluginClient: pluginClient,
		task:         taskState.TaskConfig,
		procState:    drivers.TaskStateRunning,
		startedAt:    taskState.StartedAt,
		exitCh:       make(chan struct{}),
	}

	d.tasks.Set(taskState.TaskConfig.ID, h)

	go h.run()
	return nil
}

func (d *ExecDriver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *cstructs.DriverNetwork, error) {
	if _, ok := d.tasks.Get(cfg.ID); ok {
		return nil, nil, fmt.Errorf("task with ID '%s' already started", cfg.ID)
	}

	var driverConfig TaskConfig
	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to decode driver config: %v", err)
	}

	handle := drivers.NewTaskHandle(pluginName)
	handle.Config = cfg

	pluginLogFile := filepath.Join(cfg.TaskDir().Dir, "executor.out")
	executorConfig := &dstructs.ExecutorConfig{
		LogFile:     pluginLogFile,
		LogLevel:    "debug",
		FSIsolation: true,
	}

	// TODO: best way to pass port ranges in from client config
	exec, pluginClient, err := utils.CreateExecutor(os.Stderr, hclog.Debug, d.nomadConfig, executorConfig)
	if err != nil {
		return nil, nil, err
	}

	execCmd := &executor.ExecCommand{
		Cmd:            driverConfig.Command,
		Args:           driverConfig.Args,
		Env:            cfg.EnvList(),
		User:           cfg.User,
		ResourceLimits: true,
		TaskDir:        cfg.TaskDir().Dir,
		StdoutPath:     cfg.StdoutPath,
		StderrPath:     cfg.StderrPath,
	}

	ps, err := exec.Launch(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, nil, err
	}

	h := &execTaskHandle{
		exec:         exec,
		pid:          ps.Pid,
		pluginClient: pluginClient,
		task:         cfg,
		procState:    drivers.TaskStateRunning,
		startedAt:    time.Now().Round(time.Millisecond),
		logger:       d.logger,
		exitCh:       make(chan struct{}),
	}

	driverState := &TaskState{
		ReattachConfig: utils.ReattachConfigFromGoPlugin(pluginClient.ReattachConfig()),
		Pid:            ps.Pid,
		TaskConfig:     cfg,
		StartedAt:      h.startedAt,
	}

	if err := handle.SetDriverState(&driverState); err != nil {
		d.logger.Error("failed to start task, error setting driver state", "error", err)
		exec.Shutdown("", 0)
		pluginClient.Kill()
		return nil, nil, err
	}

	d.tasks.Set(cfg.ID, h)
	go h.run()
	return handle, nil, nil
}

func (d *ExecDriver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	ch := make(chan *drivers.ExitResult)
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}
	go d.handleWait(ctx, handle, ch)

	return ch, nil
}

func (d *ExecDriver) handleWait(ctx context.Context, handle *execTaskHandle, ch chan *drivers.ExitResult) {
	defer close(ch)

	select {
	case <-ctx.Done():
		return
	case <-d.ctx.Done():
		return
	case <-handle.exitCh:
		ch <- handle.exitResult
	}
}

func (d *ExecDriver) StopTask(taskID string, timeout time.Duration, signal string) error {
	handle, ok := d.tasks.Get(taskID)
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

func (d *ExecDriver) DestroyTask(taskID string, force bool) error {
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

func (d *ExecDriver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	handle, ok := d.tasks.Get(taskID)
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

func (d *ExecDriver) TaskStats(taskID string) (*cstructs.TaskResourceUsage, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	stats, err := handle.exec.Stats()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve stats from executor: %v", err)
	}

	return stats, nil
}

func (d *ExecDriver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

func (d *ExecDriver) SignalTask(taskID string, signal string) error {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	sig := os.Interrupt
	if s, ok := signals.SignalLookup[signal]; ok {
		d.logger.Warn("signal to send to task unknown, using SIGINT", "signal", signal, "task_id", handle.task.ID)
		sig = s
	}
	return handle.exec.Signal(sig)
}

func (d *ExecDriver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	if len(cmd) == 0 {
		return nil, fmt.Errorf("error cmd must have atleast one value")
	}
	handle, ok := d.tasks.Get(taskID)
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
