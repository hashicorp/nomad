// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

const (
	// pluginName is the name of the plugin
	pluginName = "mock_driver"

	// fingerprintPeriod is the interval at which the driver will send fingerprint responses
	fingerprintPeriod = 500 * time.Millisecond

	// taskHandleVersion is the version of task handle which this driver sets
	// and understands how to decode driver state
	taskHandleVersion = 1
)

var (
	// PluginID is the mock driver plugin metadata registered in the plugin
	// catalog.
	PluginID = loader.PluginID{
		Name:       pluginName,
		PluginType: base.PluginTypeDriver,
	}

	// PluginConfig is the mock driver factory function registered in the
	// plugin catalog.
	PluginConfig = &loader.InternalPluginConfig{
		Config:  map[string]interface{}{},
		Factory: func(ctx context.Context, l hclog.Logger) interface{} { return NewMockDriver(ctx, l) },
	}

	// pluginInfo is the response returned for the PluginInfo RPC
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDriver,
		PluginApiVersions: []string{drivers.ApiVersion010},
		PluginVersion:     "0.1.0",
		Name:              pluginName,
	}

	// configSpec is the hcl specification returned by the ConfigSchema RPC
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"fs_isolation": hclspec.NewDefault(
			hclspec.NewAttr("fs_isolation", "string", false),
			hclspec.NewLiteral(fmt.Sprintf("%q", drivers.FSIsolationNone)),
		),
		"shutdown_periodic_after": hclspec.NewDefault(
			hclspec.NewAttr("shutdown_periodic_after", "bool", false),
			hclspec.NewLiteral("false"),
		),
		"shutdown_periodic_duration": hclspec.NewAttr("shutdown_periodic_duration", "number", false),
	})

	// taskConfigSpec is the hcl specification for the driver config section of
	// a task within a job. It is returned in the TaskConfigSchema RPC
	taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"start_error":             hclspec.NewAttr("start_error", "string", false),
		"start_error_recoverable": hclspec.NewAttr("start_error_recoverable", "bool", false),
		"start_block_for":         hclspec.NewAttr("start_block_for", "string", false),
		"kill_after":              hclspec.NewAttr("kill_after", "string", false),
		"plugin_exit_after":       hclspec.NewAttr("plugin_exit_after", "string", false),
		"driver_ip":               hclspec.NewAttr("driver_ip", "string", false),
		"driver_advertise":        hclspec.NewAttr("driver_advertise", "bool", false),
		"driver_port_map":         hclspec.NewAttr("driver_port_map", "string", false),

		"run_for":                hclspec.NewAttr("run_for", "string", false),
		"exit_code":              hclspec.NewAttr("exit_code", "number", false),
		"exit_signal":            hclspec.NewAttr("exit_signal", "number", false),
		"exit_err_msg":           hclspec.NewAttr("exit_err_msg", "string", false),
		"signal_error":           hclspec.NewAttr("signal_error", "string", false),
		"stdout_string":          hclspec.NewAttr("stdout_string", "string", false),
		"stdout_repeat":          hclspec.NewAttr("stdout_repeat", "number", false),
		"stdout_repeat_duration": hclspec.NewAttr("stdout_repeat_duration", "string", false),
		"stderr_string":          hclspec.NewAttr("stderr_string", "string", false),
		"stderr_repeat":          hclspec.NewAttr("stderr_repeat", "number", false),
		"stderr_repeat_duration": hclspec.NewAttr("stderr_repeat_duration", "string", false),

		"exec_command": hclspec.NewBlock("exec_command", false, hclspec.NewObject(map[string]*hclspec.Spec{
			"run_for":                hclspec.NewAttr("run_for", "string", false),
			"exit_code":              hclspec.NewAttr("exit_code", "number", false),
			"exit_signal":            hclspec.NewAttr("exit_signal", "number", false),
			"exit_err_msg":           hclspec.NewAttr("exit_err_msg", "string", false),
			"signal_error":           hclspec.NewAttr("signal_error", "string", false),
			"stdout_string":          hclspec.NewAttr("stdout_string", "string", false),
			"stdout_repeat":          hclspec.NewAttr("stdout_repeat", "number", false),
			"stdout_repeat_duration": hclspec.NewAttr("stdout_repeat_duration", "string", false),
			"stderr_string":          hclspec.NewAttr("stderr_string", "string", false),
			"stderr_repeat":          hclspec.NewAttr("stderr_repeat", "number", false),
			"stderr_repeat_duration": hclspec.NewAttr("stderr_repeat_duration", "string", false),
		})),
	})
)

// Driver is a mock DriverPlugin implementation
type Driver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *eventer.Eventer

	// capabilities is returned by the Capabilities RPC and indicates what
	// optional features this driver supports
	capabilities *drivers.Capabilities

	// config is the driver configuration set by the SetConfig RPC
	config *Config

	// tasks is the in memory datastore mapping taskIDs to mockDriverHandles
	tasks *taskStore

	// ctx is the context for the driver. It is passed to other subsystems to
	// coordinate shutdown
	ctx context.Context

	shutdownFingerprintTime time.Time

	// lastDriverTaskConfig is the last *drivers.TaskConfig passed to StartTask
	lastDriverTaskConfig *drivers.TaskConfig

	// lastTaskConfig is the last decoded *TaskConfig created by StartTask
	lastTaskConfig *TaskConfig

	// lastMu guards access to last[Driver]TaskConfig
	lastMu sync.Mutex

	// logger will log to the Nomad agent
	logger hclog.Logger
}

// NewMockDriver returns a new DriverPlugin implementation
func NewMockDriver(ctx context.Context, logger hclog.Logger) drivers.DriverPlugin {
	logger = logger.Named(pluginName)

	capabilities := &drivers.Capabilities{
		SendSignals:  true,
		Exec:         true,
		FSIsolation:  drivers.FSIsolationNone,
		MountConfigs: drivers.MountConfigSupportNone,
	}

	return &Driver{
		eventer:      eventer.NewEventer(ctx, logger),
		capabilities: capabilities,
		config:       &Config{},
		tasks:        newTaskStore(),
		ctx:          ctx,
		logger:       logger,
	}
}

// Config is the configuration for the driver that applies to all tasks
type Config struct {
	FSIsolation string `codec:"fs_isolation"`

	// ShutdownPeriodicAfter is a toggle that can be used during tests to
	// "stop" a previously-functioning driver, allowing for testing of periodic
	// drivers and fingerprinters
	ShutdownPeriodicAfter bool `codec:"shutdown_periodic_after"`

	// ShutdownPeriodicDuration is a option that can be used during tests
	// to "stop" a previously functioning driver after the specified duration
	// for testing of periodic drivers and fingerprinters.
	ShutdownPeriodicDuration time.Duration `codec:"shutdown_periodic_duration"`
}

type Command struct {
	// RunFor is the duration for which the fake task runs for. After this
	// period the MockDriver responds to the task running indicating that the
	// task has terminated
	RunFor         string `codec:"run_for"`
	runForDuration time.Duration

	// ExitCode is the exit code with which the MockDriver indicates the task
	// has exited
	ExitCode int `codec:"exit_code"`

	// ExitSignal is the signal with which the MockDriver indicates the task has
	// been killed
	ExitSignal int `codec:"exit_signal"`

	// ExitErrMsg is the error message that the task returns while exiting
	ExitErrMsg string `codec:"exit_err_msg"`

	// SignalErr is the error message that the task returns if signalled
	SignalErr string `codec:"signal_error"`

	// StdoutString is the string that should be sent to stdout
	StdoutString string `codec:"stdout_string"`

	// StdoutRepeat is the number of times the output should be sent.
	StdoutRepeat int `codec:"stdout_repeat"`

	// StdoutRepeatDur is the duration between repeated outputs.
	StdoutRepeatDur      string `codec:"stdout_repeat_duration"`
	stdoutRepeatDuration time.Duration

	// StderrString is the string that should be sent to stderr
	StderrString string `codec:"stderr_string"`

	// StderrRepeat is the number of times the errput should be sent.
	StderrRepeat int `codec:"stderr_repeat"`

	// StderrRepeatDur is the duration between repeated errputs.
	StderrRepeatDur      string `codec:"stderr_repeat_duration"`
	stderrRepeatDuration time.Duration
}

// TaskConfig is the driver configuration of a task within a job
type TaskConfig struct {
	Command

	ExecCommand *Command `codec:"exec_command"`

	// PluginExitAfter is the duration after which the mock driver indicates the
	// plugin has exited via the WaitTask call.
	PluginExitAfter         string `codec:"plugin_exit_after"`
	pluginExitAfterDuration time.Duration

	// StartErr specifies the error that should be returned when starting the
	// mock driver.
	StartErr string `codec:"start_error"`

	// StartErrRecoverable marks the error returned is recoverable
	StartErrRecoverable bool `codec:"start_error_recoverable"`

	// StartBlockFor specifies a duration in which to block before returning
	StartBlockFor         string `codec:"start_block_for"`
	startBlockForDuration time.Duration

	// KillAfter is the duration after which the mock driver indicates the task
	// has exited after getting the initial SIGINT signal
	KillAfter         string `codec:"kill_after"`
	killAfterDuration time.Duration

	// DriverIP will be returned as the DriverNetwork.IP from Start()
	DriverIP string `codec:"driver_ip"`

	// DriverAdvertise will be returned as DriverNetwork.AutoAdvertise from
	// Start().
	DriverAdvertise bool `codec:"driver_advertise"`

	// DriverPortMap will parse a label:number pair and return it in
	// DriverNetwork.PortMap from Start().
	DriverPortMap string `codec:"driver_port_map"`
}

type MockTaskState struct {
	StartedAt time.Time

	// these are not strictly "state" but because there's no external
	// reattachment we need somewhere to stash this config so we can properly
	// restore mock tasks
	Command         Command
	ExecCommand     *Command
	PluginExitAfter time.Duration
	KillAfter       time.Duration
	ProcState       drivers.TaskState
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

	d.config = &config
	if d.config.ShutdownPeriodicAfter {
		d.shutdownFingerprintTime = time.Now().Add(d.config.ShutdownPeriodicDuration)
	}

	isolation := config.FSIsolation
	if isolation != "" {
		d.capabilities.FSIsolation = drivers.FSIsolation(isolation)
	}

	return nil
}

func (d *Driver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

func (d *Driver) Capabilities() (*drivers.Capabilities, error) {
	return d.capabilities, nil
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
	var health drivers.HealthState
	var desc string
	attrs := map[string]*pstructs.Attribute{}
	if !d.shutdownFingerprintTime.IsZero() && time.Now().After(d.shutdownFingerprintTime) {
		health = drivers.HealthStateUndetected
		desc = "disabled"
	} else {
		health = drivers.HealthStateHealthy
		attrs["driver.mock"] = pstructs.NewBoolAttribute(true)
		desc = drivers.DriverHealthy
	}

	return &drivers.Fingerprint{
		Attributes:        attrs,
		Health:            health,
		HealthDescription: desc,
	}
}

func (d *Driver) RecoverTask(handle *drivers.TaskHandle) error {
	if handle == nil {
		return fmt.Errorf("handle cannot be nil")
	}

	// Unmarshall the driver state and create a new handle
	var taskState MockTaskState
	if err := handle.GetDriverState(&taskState); err != nil {
		d.logger.Error("failed to decode task state from handle", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to decode task state from handle: %v", err)
	}

	taskState.Command.parseDurations()
	if taskState.ExecCommand != nil {
		taskState.ExecCommand.parseDurations()
	}

	// Correct the run_for time based on how long it has already been running
	now := time.Now()
	if !taskState.StartedAt.IsZero() {
		taskState.Command.runForDuration = taskState.Command.runForDuration - now.Sub(taskState.StartedAt)

		if taskState.ExecCommand != nil {
			taskState.ExecCommand.runForDuration = taskState.ExecCommand.runForDuration - now.Sub(taskState.StartedAt)
		}
	}

	// Recreate the taskHandle. Because there's no real running process, we'll
	// assume we're still running if we've recovered it at all.
	killCtx, killCancel := context.WithCancel(context.Background())
	h := &taskHandle{
		logger:          d.logger.With("task_name", handle.Config.Name),
		pluginExitAfter: taskState.PluginExitAfter,
		killAfter:       taskState.KillAfter,
		waitCh:          make(chan any),
		taskConfig:      handle.Config,
		command:         taskState.Command,
		execCommand:     taskState.ExecCommand,
		procState:       drivers.TaskStateRunning,
		startedAt:       taskState.StartedAt,
		kill:            killCancel,
		killCh:          killCtx.Done(),
		Recovered:       true,
	}

	d.tasks.Set(handle.Config.ID, h)
	go h.run()
	return nil
}

func (c *Command) parseDurations() error {
	var err error
	if c.runForDuration, err = parseDuration(c.RunFor); err != nil {
		return fmt.Errorf("run_for %v not a valid duration: %v", c.RunFor, err)
	}

	if c.stdoutRepeatDuration, err = parseDuration(c.StdoutRepeatDur); err != nil {
		return fmt.Errorf("stdout_repeat_duration %v not a valid duration: %v", c.stdoutRepeatDuration, err)
	}

	if c.stderrRepeatDuration, err = parseDuration(c.StderrRepeatDur); err != nil {
		return fmt.Errorf("stderr_repeat_duration %v not a valid duration: %v", c.stderrRepeatDuration, err)
	}

	return nil
}

func parseDriverConfig(cfg *drivers.TaskConfig) (*TaskConfig, error) {
	var driverConfig TaskConfig
	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, err
	}

	var err error
	if driverConfig.startBlockForDuration, err = parseDuration(driverConfig.StartBlockFor); err != nil {
		return nil, fmt.Errorf("start_block_for %v not a valid duration: %v", driverConfig.StartBlockFor, err)
	}

	if driverConfig.pluginExitAfterDuration, err = parseDuration(driverConfig.PluginExitAfter); err != nil {
		return nil, fmt.Errorf("plugin_exit_after %v not a valid duration: %v", driverConfig.PluginExitAfter, err)
	}

	if err = driverConfig.parseDurations(); err != nil {
		return nil, err
	}

	if driverConfig.ExecCommand != nil {
		if err = driverConfig.ExecCommand.parseDurations(); err != nil {
			return nil, err
		}
	}

	return &driverConfig, nil
}

func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	driverConfig, err := parseDriverConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	if driverConfig.startBlockForDuration != 0 {
		time.Sleep(driverConfig.startBlockForDuration)
	}

	// Store last configs
	d.lastMu.Lock()
	d.lastDriverTaskConfig = cfg
	d.lastTaskConfig = driverConfig
	d.lastMu.Unlock()

	if driverConfig.StartErr != "" {
		return nil, nil, structs.NewRecoverableError(errors.New(driverConfig.StartErr), driverConfig.StartErrRecoverable)
	}

	// Create the driver network
	net := &drivers.DriverNetwork{
		IP:            driverConfig.DriverIP,
		AutoAdvertise: driverConfig.DriverAdvertise,
	}
	if raw := driverConfig.DriverPortMap; len(raw) > 0 {
		parts := strings.Split(raw, ":")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("malformed port map: %q", raw)
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, nil, fmt.Errorf("malformed port map: %q -- error: %v", raw, err)
		}
		net.PortMap = map[string]int{parts[0]: port}
	}

	killCtx, killCancel := context.WithCancel(context.Background())
	h := &taskHandle{
		taskConfig:      cfg,
		command:         driverConfig.Command,
		execCommand:     driverConfig.ExecCommand,
		pluginExitAfter: driverConfig.pluginExitAfterDuration,
		killAfter:       driverConfig.killAfterDuration,
		logger:          d.logger.With("task_name", cfg.Name),
		waitCh:          make(chan interface{}),
		killCh:          killCtx.Done(),
		kill:            killCancel,
		startedAt:       time.Now(),
	}

	driverState := MockTaskState{
		StartedAt:       h.startedAt,
		Command:         driverConfig.Command,
		ExecCommand:     driverConfig.ExecCommand,
		PluginExitAfter: driverConfig.pluginExitAfterDuration,
		KillAfter:       driverConfig.killAfterDuration,
	}
	handle := drivers.NewTaskHandle(taskHandleVersion)
	handle.Config = cfg
	if err := handle.SetDriverState(&driverState); err != nil {
		d.logger.Error("failed to start task, error setting driver state", "error", err, "task_name", cfg.Name)
		return nil, nil, fmt.Errorf("failed to set driver state: %v", err)
	}

	d.tasks.Set(cfg.ID, h)

	d.logger.Debug("starting task", "task_name", cfg.Name)
	go h.run()
	return handle, net, nil

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
func (d *Driver) handleWait(ctx context.Context, handle *taskHandle, ch chan *drivers.ExitResult) {
	defer close(ch)

	select {
	case <-ctx.Done():
		return
	case <-d.ctx.Done():
		return
	case <-handle.waitCh:
		ch <- handle.exitResult
	}
}
func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	d.logger.Debug("killing task", "task_name", h.taskConfig.Name, "kill_after", h.killAfter)

	select {
	case <-h.waitCh:
		d.logger.Debug("not killing task: already exited", "task_name", h.taskConfig.Name)
	case <-time.After(h.killAfter):
		d.logger.Debug("killing task due to kill_after", "task_name", h.taskConfig.Name)
		h.kill()
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

	d.tasks.Delete(taskID)
	return nil
}

func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	return h.TaskStatus(), nil

}

func (d *Driver) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	ch := make(chan *drivers.TaskResourceUsage)
	go d.handleStats(ctx, ch)
	return ch, nil
}

func (d *Driver) handleStats(ctx context.Context, ch chan<- *drivers.TaskResourceUsage) {
	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			// Generate random value for the memory usage
			s := &drivers.TaskResourceUsage{
				ResourceUsage: &drivers.ResourceUsage{
					MemoryStats: &drivers.MemoryStats{
						RSS:      rand.Uint64(),
						Measured: []string{"RSS"},
					},
				},
				Timestamp: time.Now().UTC().UnixNano(),
			}
			select {
			case <-ctx.Done():
				return
			case ch <- s:
			default:
			}
		case <-ctx.Done():
			return
		}
	}
}

func (d *Driver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

func (d *Driver) SignalTask(taskID string, signal string) error {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	if h.command.SignalErr == "" {
		return nil
	}

	return errors.New(h.command.SignalErr)
}

func (d *Driver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	res := drivers.ExecTaskResult{
		Stdout:     []byte(fmt.Sprintf("Exec(%q, %q)", h.taskConfig.Name, cmd)),
		ExitResult: &drivers.ExitResult{},
	}
	return &res, nil
}

var _ drivers.ExecTaskStreamingDriver = (*Driver)(nil)

func (d *Driver) ExecTaskStreaming(ctx context.Context, taskID string, execOpts *drivers.ExecOptions) (*drivers.ExitResult, error) {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	d.logger.Info("executing task", "command", h.execCommand, "task_id", taskID)

	if h.execCommand == nil {
		return nil, errors.New("no exec command is configured")
	}

	cancelCh := make(chan struct{})
	exitTimer := make(chan time.Time)

	cmd := *h.execCommand
	if len(execOpts.Command) == 1 && execOpts.Command[0] == "showinput" {
		stdin, _ := io.ReadAll(execOpts.Stdin)
		cmd = Command{
			RunFor: "1ms",
			StdoutString: fmt.Sprintf("TTY: %v\nStdin:\n%s\n",
				execOpts.Tty,
				stdin,
			),
		}
	}

	return runCommand(cmd, execOpts.Stdout, execOpts.Stderr, cancelCh, exitTimer, d.logger), nil
}

// GetTaskConfig is unique to the mock driver and for testing purposes only. It
// returns the *drivers.TaskConfig passed to StartTask and the decoded
// *mock.TaskConfig created by the last StartTask call.
func (d *Driver) GetTaskConfig() (*drivers.TaskConfig, *TaskConfig) {
	d.lastMu.Lock()
	defer d.lastMu.Unlock()
	return d.lastDriverTaskConfig, d.lastTaskConfig
}

// GetHandle is unique to the mock driver and for testing purposes only. It
// returns the handle of the given task ID
func (d *Driver) GetHandle(taskID string) *taskHandle {
	h, _ := d.tasks.Get(taskID)
	return h
}

var _ drivers.DriverNetworkManager = (*Driver)(nil)

func (d *Driver) CreateNetwork(allocID string, request *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error) {
	return nil, true, nil
}

func (d *Driver) DestroyNetwork(allocID string, spec *drivers.NetworkIsolationSpec) error {
	return nil
}
