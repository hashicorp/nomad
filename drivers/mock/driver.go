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
)

const (
	// pluginName is the name of the plugin
	pluginName = "mock"

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
		SendSignals: true,
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

	// logger will log to the plugin output which is usually an 'executor.out'
	// file located in the root of the TaskDir
	logger hclog.Logger
}

// Config is the configuration for the driver that applies to all tasks
type Config struct {
	// ShutdownPeriodicAfter is a toggle that can be used during tests to
	// "stop" a previously-functioning driver, allowing for testing of periodic
	// drivers and fingerprinters
	ShutdownPeriodicAfter bool `cty:"shutdown_periodic_after"`

	// ShutdownPeriodicDuration is a option that can be used during tests
	// to "stop" a previously functioning driver after the specified duration
	// for testing of periodic drivers and fingerprinters.
	ShutdownPeriodicDuration time.Duration `cty:"shutdown_periodic_duration"`
}

// TaskConfig is the driver configuration of a task within a job
type TaskConfig struct {

	// StartErr specifies the error that should be returned when starting the
	// mock driver.
	StartErr string `cty:"start_error"`

	// StartErrRecoverable marks the error returned is recoverable
	StartErrRecoverable bool `cty:"start_error_recoverable"`

	// StartBlockFor specifies a duration in which to block before returning
	StartBlockFor time.Duration `cty:"start_block_for"`

	// KillAfter is the duration after which the mock driver indicates the task
	// has exited after getting the initial SIGINT signal
	KillAfter time.Duration `cty:"kill_after"`

	// RunFor is the duration for which the fake task runs for. After this
	// period the MockDriver responds to the task running indicating that the
	// task has terminated
	RunFor time.Duration `cty:"run_for"`

	// ExitCode is the exit code with which the MockDriver indicates the task
	// has exited
	ExitCode int `cty:"exit_code"`

	// ExitSignal is the signal with which the MockDriver indicates the task has
	// been killed
	ExitSignal int `cty:"exit_signal"`

	// ExitErrMsg is the error message that the task returns while exiting
	ExitErrMsg string `cty:"exit_err_msg"`

	// SignalErr is the error message that the task returns if signalled
	SignalErr string `cty:"signal_error"`

	// DriverIP will be returned as the DriverNetwork.IP from Start()
	DriverIP string `cty:"driver_ip"`

	// DriverAdvertise will be returned as DriverNetwork.AutoAdvertise from
	// Start().
	DriverAdvertise bool `cty:"driver_advertise"`

	// DriverPortMap will parse a label:number pair and return it in
	// DriverNetwork.PortMap from Start().
	DriverPortMap string `cty:"driver_port_map"`

	// StdoutString is the string that should be sent to stdout
	StdoutString string `cty:"stdout_string"`

	// StdoutRepeat is the number of times the output should be sent.
	StdoutRepeat int `cty:"stdout_repeat"`

	// StdoutRepeatDur is the duration between repeated outputs.
	StdoutRepeatDur time.Duration `cty:"stdout_repeat_duration"`
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

func (d *Driver) SetConfig(data []byte) error {
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
	attrs := map[string]string{}
	if !d.shutdownFingerprintTime.IsZero() && time.Now().After(d.shutdownFingerprintTime) {
		health = drivers.HealthStateUndetected
		desc = "mock disabled"
	} else {
		health = drivers.HealthStateHealthy
		attrs["driver.mock"] = "1"
		desc = "mock enabled"
	}

	return &drivers.Fingerprint{
		Attributes:        map[string]string{},
		Health:            health,
		HealthDescription: desc,
	}
}

func (d *Driver) RecoverTask(*drivers.TaskHandle) error {
	panic("not implemented")
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

	h := &mockTaskHandle{
		task:            cfg,
		runFor:          driverConfig.RunFor,
		killAfter:       driverConfig.KillAfter,
		exitCode:        driverConfig.ExitCode,
		exitSignal:      driverConfig.ExitSignal,
		stdoutString:    driverConfig.StdoutString,
		stdoutRepeat:    driverConfig.StdoutRepeat,
		stdoutRepeatDur: driverConfig.StdoutRepeatDur,
		logger:          d.logger,
		doneCh:          make(chan struct{}),
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
		d.logger.Error("failed to start task, error setting driver state", "error", err)
		return nil, nil, fmt.Errorf("failed to set driver state: %v", err)
	}

	d.tasks.Set(cfg.ID, h)

	d.logger.Debug("starting task", "name", cfg.Name)
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
func (d *Driver) handleWait(ctx context.Context, handle *mockTaskHandle, ch chan *drivers.ExitResult) {
	defer close(ch)

	select {
	case <-ctx.Done():
		return
	case <-d.ctx.Done():
		return
	case <-handle.doneCh:
		ch <- handle.exitResult
	}
}
func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
	panic("not implemented")
}

func (d *Driver) DestroyTask(taskID string, force bool) error {
	panic("not implemented")
}

func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	panic("not implemented")
}

func (d *Driver) TaskStats(taskID string) (*cstructs.TaskResourceUsage, error) {
	panic("not implemented")
}

func (d *Driver) TaskEvents(context.Context) (<-chan *drivers.TaskEvent, error) {
	panic("not implemented")
}

func (d *Driver) SignalTask(taskID string, signal string) error {
	panic("not implemented")
}

func (d *Driver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	panic("not implemented")
}
