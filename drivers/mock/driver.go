package mock

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/loader"
	netctx "golang.org/x/net/context"
)

const (
	// pluginName is the name of the plugin
	pluginName = "mock_driver"

	// fingerprintPeriod is the interval at which the driver will send fingerprint responses
	fingerprintPeriod = 500 * time.Millisecond
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
		Factory: func(l hclog.Logger) interface{} { return NewMockDriver(l) },
	}

	// pluginInfo is the response returned for the PluginInfo RPC
	pluginInfo = &base.PluginInfoResponse{
		Type:             base.PluginTypeDriver,
		PluginApiVersion: "0.0.1",
		PluginVersion:    "0.1.0",
		Name:             pluginName,
	}

	// configSpec is the hcl specification returned by the ConfigSchema RPC
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
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
		"start_block_for":         hclspec.NewAttr("start_block_for", "number", false),
		"kill_after":              hclspec.NewAttr("kill_after", "number", false),
		"run_for":                 hclspec.NewAttr("run_for", "number", false),
		"exit_code":               hclspec.NewAttr("exit_code", "number", false),
		"exit_signal":             hclspec.NewAttr("exit_signal", "number", false),
		"exit_err_msg":            hclspec.NewAttr("exit_err_msg", "string", false),
		"signal_err":              hclspec.NewAttr("signal_err", "string", false),
		"driver_ip":               hclspec.NewAttr("driver_ip", "string", false),
		"driver_advertise":        hclspec.NewAttr("driver_advertise", "bool", false),
		"driver_port_map":         hclspec.NewAttr("driver_port_map", "string", false),
		"stdout_string":           hclspec.NewAttr("stdout_string", "string", false),
		"stdout_repeat":           hclspec.NewAttr("stdout_repeat", "number", false),
		"stdout_repeat_duration":  hclspec.NewAttr("stdout_repeat_duration", "number", false),
	})

	// capabilities is returned by the Capabilities RPC and indicates what
	// optional features this driver supports
	capabilities = &drivers.Capabilities{
		SendSignals: false,
		Exec:        true,
		FSIsolation: cstructs.FSIsolationNone,
	}
)

// Driver is a mock DriverPlugin implementation
type Driver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *eventer.Eventer

	// config is the driver configuration set by the SetConfig RPC
	config *Config

	// tasks is the in memory datastore mapping taskIDs to mockDriverHandles
	tasks *taskStore

	// ctx is the context for the driver. It is passed to other subsystems to
	// coordinate shutdown
	ctx context.Context

	// signalShutdown is called when the driver is shutting down and cancels the
	// ctx passed to any subsystems
	signalShutdown context.CancelFunc

	shutdownFingerprintTime time.Time

	// logger will log to the Nomad agent
	logger hclog.Logger
}

// NewMockDriver returns a new DriverPlugin implementation
func NewMockDriver(logger hclog.Logger) drivers.DriverPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	logger = logger.Named(pluginName)
	return &Driver{
		eventer:        eventer.NewEventer(ctx, logger),
		config:         &Config{},
		tasks:          newTaskStore(),
		ctx:            ctx,
		signalShutdown: cancel,
		logger:         logger,
	}
}

// Config is the configuration for the driver that applies to all tasks
type Config struct {
	// ShutdownPeriodicAfter is a toggle that can be used during tests to
	// "stop" a previously-functioning driver, allowing for testing of periodic
	// drivers and fingerprinters
	ShutdownPeriodicAfter bool `codec:"shutdown_periodic_after"`

	// ShutdownPeriodicDuration is a option that can be used during tests
	// to "stop" a previously functioning driver after the specified duration
	// for testing of periodic drivers and fingerprinters.
	ShutdownPeriodicDuration time.Duration `codec:"shutdown_periodic_duration"`
}

// TaskConfig is the driver configuration of a task within a job
type TaskConfig struct {

	// StartErr specifies the error that should be returned when starting the
	// mock driver.
	StartErr string `codec:"start_error"`

	// StartErrRecoverable marks the error returned is recoverable
	StartErrRecoverable bool `codec:"start_error_recoverable"`

	// StartBlockFor specifies a duration in which to block before returning
	StartBlockFor time.Duration `codec:"start_block_for"`

	// KillAfter is the duration after which the mock driver indicates the task
	// has exited after getting the initial SIGINT signal
	KillAfter time.Duration `codec:"kill_after"`

	// RunFor is the duration for which the fake task runs for. After this
	// period the MockDriver responds to the task running indicating that the
	// task has terminated
	RunFor time.Duration `codec:"run_for"`

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

	// DriverIP will be returned as the DriverNetwork.IP from Start()
	DriverIP string `codec:"driver_ip"`

	// DriverAdvertise will be returned as DriverNetwork.AutoAdvertise from
	// Start().
	DriverAdvertise bool `codec:"driver_advertise"`

	// DriverPortMap will parse a label:number pair and return it in
	// DriverNetwork.PortMap from Start().
	DriverPortMap string `codec:"driver_port_map"`

	// StdoutString is the string that should be sent to stdout
	StdoutString string `codec:"stdout_string"`

	// StdoutRepeat is the number of times the output should be sent.
	StdoutRepeat int `codec:"stdout_repeat"`

	// StdoutRepeatDur is the duration between repeated outputs.
	StdoutRepeatDur time.Duration `codec:"stdout_repeat_duration"`
}

type MockTaskState struct {
	TaskConfig *drivers.TaskConfig
	StartedAt  time.Time
}

func (d *Driver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

func (d *Driver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

func (d *Driver) SetConfig(data []byte, cfg *base.ClientAgentConfig) error {
	var config Config
	if err := base.MsgPackDecode(data, &config); err != nil {
		return err
	}

	d.config = &config
	if d.config.ShutdownPeriodicAfter {
		d.shutdownFingerprintTime = time.Now().Add(d.config.ShutdownPeriodicDuration)
	}
	return nil
}

func (d *Driver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

func (d *Driver) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}

func (d *Driver) Fingerprint(ctx netctx.Context) (<-chan *drivers.Fingerprint, error) {
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
	attrs := map[string]string{}
	if !d.shutdownFingerprintTime.IsZero() && time.Now().After(d.shutdownFingerprintTime) {
		health = drivers.HealthStateUndetected
		desc = "disabled"
	} else {
		health = drivers.HealthStateHealthy
		attrs["driver.mock"] = "1"
		desc = "ready"
	}

	return &drivers.Fingerprint{
		Attributes:        map[string]string{},
		Health:            health,
		HealthDescription: desc,
	}
}

func (d *Driver) RecoverTask(h *drivers.TaskHandle) error {
	if h == nil {
		return fmt.Errorf("handle cannot be nil")
	}

	if _, ok := d.tasks.Get(h.Config.ID); ok {
		d.logger.Debug("nothing to recover; task already exists",
			"task_id", h.Config.ID,
			"task_name", h.Config.Name,
		)
		return nil
	}

	// Recovering a task requires the task to be running external to the
	// plugin. Since the mock_driver runs all tasks in process it cannot
	// recover tasks.
	return fmt.Errorf("%s cannot recover tasks", pluginName)
}

func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *cstructs.DriverNetwork, error) {
	var driverConfig TaskConfig
	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, nil, err
	}

	if driverConfig.StartBlockFor != 0 {
		time.Sleep(driverConfig.StartBlockFor)
	}

	if driverConfig.StartErr != "" {
		return nil, nil, structs.NewRecoverableError(errors.New(driverConfig.StartErr), driverConfig.StartErrRecoverable)
	}

	// Create the driver network
	net := &cstructs.DriverNetwork{
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
		runFor:          driverConfig.RunFor,
		killAfter:       driverConfig.KillAfter,
		exitCode:        driverConfig.ExitCode,
		exitSignal:      driverConfig.ExitSignal,
		stdoutString:    driverConfig.StdoutString,
		stdoutRepeat:    driverConfig.StdoutRepeat,
		stdoutRepeatDur: driverConfig.StdoutRepeatDur,
		logger:          d.logger.With("task_name", cfg.Name),
		waitCh:          make(chan struct{}),
		killCh:          killCtx.Done(),
		kill:            killCancel,
	}
	if driverConfig.ExitErrMsg != "" {
		h.exitErr = errors.New(driverConfig.ExitErrMsg)
	}
	if driverConfig.SignalErr != "" {
		h.signalErr = fmt.Errorf(driverConfig.SignalErr)
	}

	driverState := MockTaskState{
		TaskConfig: cfg,
		StartedAt:  h.startedAt,
	}
	handle := drivers.NewTaskHandle(pluginName)
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

func (d *Driver) WaitTask(ctx netctx.Context, taskID string) (<-chan *drivers.ExitResult, error) {
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
	panic("not implemented")
}

func (d *Driver) TaskStats(taskID string) (*cstructs.TaskResourceUsage, error) {
	//TODO return an error?
	return nil, nil
}

func (d *Driver) TaskEvents(ctx netctx.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

func (d *Driver) SignalTask(taskID string, signal string) error {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	return h.signalErr
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
