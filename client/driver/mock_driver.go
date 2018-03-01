package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/nomad/client/driver/logging"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// ShutdownPeriodicAfter is a config key that can be used during tests to
	// "stop" a previously-functioning driver, allowing for testing of periodic
	// drivers and fingerprinters
	ShutdownPeriodicAfter = "test.shutdown_periodic_after"

	// ShutdownPeriodicDuration is a config option that can be used during tests
	// to "stop" a previously functioning driver after the specified duration
	// (specified in seconds) for testing of periodic drivers and fingerprinters.
	ShutdownPeriodicDuration = "test.shutdown_periodic_duration"

	mockDriverName = "driver.mock_driver"
)

// MockDriverConfig is the driver configuration for the MockDriver
type MockDriverConfig struct {

	// StartErr specifies the error that should be returned when starting the
	// mock driver.
	StartErr string `mapstructure:"start_error"`

	// StartErrRecoverable marks the error returned is recoverable
	StartErrRecoverable bool `mapstructure:"start_error_recoverable"`

	// StartBlockFor specifies a duration in which to block before returning
	StartBlockFor time.Duration `mapstructure:"start_block_for"`

	// KillAfter is the duration after which the mock driver indicates the task
	// has exited after getting the initial SIGINT signal
	KillAfter time.Duration `mapstructure:"kill_after"`

	// RunFor is the duration for which the fake task runs for. After this
	// period the MockDriver responds to the task running indicating that the
	// task has terminated
	RunFor time.Duration `mapstructure:"run_for"`

	// ExitCode is the exit code with which the MockDriver indicates the task
	// has exited
	ExitCode int `mapstructure:"exit_code"`

	// ExitSignal is the signal with which the MockDriver indicates the task has
	// been killed
	ExitSignal int `mapstructure:"exit_signal"`

	// ExitErrMsg is the error message that the task returns while exiting
	ExitErrMsg string `mapstructure:"exit_err_msg"`

	// SignalErr is the error message that the task returns if signalled
	SignalErr string `mapstructure:"signal_error"`

	// DriverIP will be returned as the DriverNetwork.IP from Start()
	DriverIP string `mapstructure:"driver_ip"`

	// DriverAdvertise will be returned as DriverNetwork.AutoAdvertise from
	// Start().
	DriverAdvertise bool `mapstructure:"driver_advertise"`

	// DriverPortMap will parse a label:number pair and return it in
	// DriverNetwork.PortMap from Start().
	DriverPortMap string `mapstructure:"driver_port_map"`

	// StdoutString is the string that should be sent to stdout
	StdoutString string `mapstructure:"stdout_string"`

	// StdoutRepeat is the number of times the output should be sent.
	StdoutRepeat int `mapstructure:"stdout_repeat"`

	// StdoutRepeatDur is the duration between repeated outputs.
	StdoutRepeatDur time.Duration `mapstructure:"stdout_repeat_duration"`
}

// MockDriver is a driver which is used for testing purposes
type MockDriver struct {
	DriverContext

	cleanupFailNum int

	// shutdownFingerprintTime is the time up to which the driver will be up
	shutdownFingerprintTime time.Time
}

// NewMockDriver is a factory method which returns a new Mock Driver
func NewMockDriver(ctx *DriverContext) Driver {
	md := &MockDriver{DriverContext: *ctx}

	// if the shutdown configuration options are set, start the timer here.
	// This config option defaults to false
	if ctx.config != nil && ctx.config.ReadBoolDefault(ShutdownPeriodicAfter, false) {
		duration, err := ctx.config.ReadInt(ShutdownPeriodicDuration)
		if err != nil {
			errMsg := fmt.Sprintf("unable to read config option for shutdown_periodic_duration %v, got err %s", duration, err.Error())
			panic(errMsg)
		}
		md.shutdownFingerprintTime = time.Now().Add(time.Second * time.Duration(duration))
	}

	return md
}

func (d *MockDriver) Abilities() DriverAbilities {
	return DriverAbilities{
		SendSignals: false,
		Exec:        true,
	}
}

func (d *MockDriver) FSIsolation() cstructs.FSIsolation {
	return cstructs.FSIsolationNone
}

func (d *MockDriver) Prestart(*ExecContext, *structs.Task) (*PrestartResponse, error) {
	return nil, nil
}

// Start starts the mock driver
func (m *MockDriver) Start(ctx *ExecContext, task *structs.Task) (*StartResponse, error) {
	var driverConfig MockDriverConfig
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           &driverConfig,
	})
	if err != nil {
		return nil, err
	}
	if err := dec.Decode(task.Config); err != nil {
		return nil, err
	}

	if driverConfig.StartBlockFor != 0 {
		time.Sleep(driverConfig.StartBlockFor)
	}

	if driverConfig.StartErr != "" {
		return nil, structs.NewRecoverableError(errors.New(driverConfig.StartErr), driverConfig.StartErrRecoverable)
	}

	// Create the driver network
	net := &cstructs.DriverNetwork{
		IP:            driverConfig.DriverIP,
		AutoAdvertise: driverConfig.DriverAdvertise,
	}
	if raw := driverConfig.DriverPortMap; len(raw) > 0 {
		parts := strings.Split(raw, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed port map: %q", raw)
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("malformed port map: %q -- error: %v", raw, err)
		}
		net.PortMap = map[string]int{parts[0]: port}
	}

	h := mockDriverHandle{
		ctx:             ctx,
		task:            task,
		taskName:        task.Name,
		runFor:          driverConfig.RunFor,
		killAfter:       driverConfig.KillAfter,
		killTimeout:     task.KillTimeout,
		exitCode:        driverConfig.ExitCode,
		exitSignal:      driverConfig.ExitSignal,
		stdoutString:    driverConfig.StdoutString,
		stdoutRepeat:    driverConfig.StdoutRepeat,
		stdoutRepeatDur: driverConfig.StdoutRepeatDur,
		logger:          m.logger,
		doneCh:          make(chan struct{}),
		waitCh:          make(chan *dstructs.WaitResult, 1),
	}
	if driverConfig.ExitErrMsg != "" {
		h.exitErr = errors.New(driverConfig.ExitErrMsg)
	}
	if driverConfig.SignalErr != "" {
		h.signalErr = fmt.Errorf(driverConfig.SignalErr)
	}
	m.logger.Printf("[DEBUG] driver.mock: starting task %q", task.Name)
	go h.run()

	return &StartResponse{Handle: &h, Network: net}, nil
}

// Cleanup deletes all keys except for Config.Options["cleanup_fail_on"] for
// Config.Options["cleanup_fail_num"] times. For failures it will return a
// recoverable error.
func (m *MockDriver) Cleanup(ctx *ExecContext, res *CreatedResources) error {
	if res == nil {
		panic("Cleanup should not be called with nil *CreatedResources")
	}

	var err error
	failn, _ := strconv.Atoi(m.config.Options["cleanup_fail_num"])
	failk := m.config.Options["cleanup_fail_on"]
	for k := range res.Resources {
		if k == failk && m.cleanupFailNum < failn {
			m.cleanupFailNum++
			err = structs.NewRecoverableError(fmt.Errorf("mock_driver failure on %q call %d/%d", k, m.cleanupFailNum, failn), true)
		} else {
			delete(res.Resources, k)
		}
	}
	return err
}

// Validate validates the mock driver configuration
func (m *MockDriver) Validate(map[string]interface{}) error {
	return nil
}

// Fingerprint fingerprints a node and returns if MockDriver is enabled
func (m *MockDriver) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	switch {
	// If the driver is configured to shut down after a period of time, and the
	// current time is after the time which the node should shut down, simulate
	// driver failure
	case !m.shutdownFingerprintTime.IsZero() && time.Now().After(m.shutdownFingerprintTime):
		resp.RemoveAttribute(mockDriverName)
	default:
		resp.AddAttribute(mockDriverName, "1")
		resp.Detected = true
	}
	return nil
}

// When testing, poll for updates
func (m *MockDriver) Periodic() (bool, time.Duration) {
	return true, 500 * time.Millisecond
}

// HealthCheck implements the interface for HealthCheck, and indicates the current
// health status of the mock driver.
func (m *MockDriver) HealthCheck(req *cstructs.HealthCheckRequest, resp *cstructs.HealthCheckResponse) error {
	switch {
	case !m.shutdownFingerprintTime.IsZero() && time.Now().After(m.shutdownFingerprintTime):
		notHealthy := &structs.DriverInfo{
			Healthy:           false,
			HealthDescription: "not running",
			UpdateTime:        time.Now(),
		}
		resp.AddDriverInfo("mock_driver", notHealthy)
		return nil
	default:
		healthy := &structs.DriverInfo{
			Healthy:           true,
			HealthDescription: "running",
			UpdateTime:        time.Now(),
		}
		resp.AddDriverInfo("mock_driver", healthy)
		return nil
	}
}

// GetHealthCheckInterval implements the interface for HealthCheck and indicates
// that mock driver should be checked periodically. Returns a boolean
// indicating if it should be checked, and the duration at which to do this
// check.
func (m *MockDriver) GetHealthCheckInterval(req *cstructs.HealthCheckIntervalRequest, resp *cstructs.HealthCheckIntervalResponse) error {
	resp.Eligible = true
	resp.Period = 1 * time.Second
	return nil
}

// MockDriverHandle is a driver handler which supervises a mock task
type mockDriverHandle struct {
	ctx             *ExecContext
	task            *structs.Task
	taskName        string
	runFor          time.Duration
	killAfter       time.Duration
	killTimeout     time.Duration
	exitCode        int
	exitSignal      int
	exitErr         error
	signalErr       error
	logger          *log.Logger
	stdoutString    string
	stdoutRepeat    int
	stdoutRepeatDur time.Duration
	waitCh          chan *dstructs.WaitResult
	doneCh          chan struct{}
}

type mockDriverID struct {
	TaskName    string
	RunFor      time.Duration
	KillAfter   time.Duration
	KillTimeout time.Duration
	ExitCode    int
	ExitSignal  int
	ExitErr     error
	SignalErr   error
}

func (h *mockDriverHandle) ID() string {
	id := mockDriverID{
		TaskName:    h.taskName,
		RunFor:      h.runFor,
		KillAfter:   h.killAfter,
		KillTimeout: h.killTimeout,
		ExitCode:    h.exitCode,
		ExitSignal:  h.exitSignal,
		ExitErr:     h.exitErr,
		SignalErr:   h.signalErr,
	}

	data, err := json.Marshal(id)
	if err != nil {
		h.logger.Printf("[ERR] driver.mock_driver: failed to marshal ID to JSON: %s", err)
	}
	return string(data)
}

// Open re-connects the driver to the running task
func (m *MockDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	id := &mockDriverID{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}

	h := mockDriverHandle{
		taskName:    id.TaskName,
		runFor:      id.RunFor,
		killAfter:   id.KillAfter,
		killTimeout: id.KillTimeout,
		exitCode:    id.ExitCode,
		exitSignal:  id.ExitSignal,
		exitErr:     id.ExitErr,
		signalErr:   id.SignalErr,
		logger:      m.logger,
		doneCh:      make(chan struct{}),
		waitCh:      make(chan *dstructs.WaitResult, 1),
	}

	go h.run()
	return &h, nil
}

func (h *mockDriverHandle) WaitCh() chan *dstructs.WaitResult {
	return h.waitCh
}

func (h *mockDriverHandle) Exec(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
	h.logger.Printf("[DEBUG] driver.mock: Exec(%q, %q)", cmd, args)
	return []byte(fmt.Sprintf("Exec(%q, %q)", cmd, args)), 0, nil
}

// TODO Implement when we need it.
func (h *mockDriverHandle) Update(task *structs.Task) error {
	h.killTimeout = task.KillTimeout
	return nil
}

// TODO Implement when we need it.
func (h *mockDriverHandle) Signal(s os.Signal) error {
	return h.signalErr
}

// Kill kills a mock task
func (h *mockDriverHandle) Kill() error {
	h.logger.Printf("[DEBUG] driver.mock: killing task %q after kill timeout: %v", h.taskName, h.killTimeout)
	select {
	case <-h.doneCh:
	case <-time.After(h.killAfter):
		select {
		case <-h.doneCh:
			// already closed
		default:
			close(h.doneCh)
		}
	case <-time.After(h.killTimeout):
		h.logger.Printf("[DEBUG] driver.mock: terminating task %q", h.taskName)
		select {
		case <-h.doneCh:
			// already closed
		default:
			close(h.doneCh)
		}
	}
	return nil
}

// TODO Implement when we need it.
func (h *mockDriverHandle) Stats() (*cstructs.TaskResourceUsage, error) {
	return nil, nil
}

// run waits for the configured amount of time and then indicates the task has
// terminated
func (h *mockDriverHandle) run() {
	// Setup logging output
	if h.stdoutString != "" {
		go h.handleLogging()
	}

	timer := time.NewTimer(h.runFor)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			select {
			case <-h.doneCh:
				// already closed
			default:
				close(h.doneCh)
			}
		case <-h.doneCh:
			h.logger.Printf("[DEBUG] driver.mock: finished running task %q", h.taskName)
			h.waitCh <- dstructs.NewWaitResult(h.exitCode, h.exitSignal, h.exitErr)
			return
		}
	}
}

// handleLogging handles logging stdout messages
func (h *mockDriverHandle) handleLogging() {
	if h.stdoutString == "" {
		return
	}

	// Setup a log rotator
	logFileSize := int64(h.task.LogConfig.MaxFileSizeMB * 1024 * 1024)
	lro, err := logging.NewFileRotator(h.ctx.TaskDir.LogDir, fmt.Sprintf("%v.stdout", h.taskName),
		h.task.LogConfig.MaxFiles, logFileSize, h.logger)
	if err != nil {
		h.exitErr = err
		close(h.doneCh)
		h.logger.Printf("[ERR] mock_driver: failed to setup file rotator: %v", err)
		return
	}
	defer lro.Close()

	// Do initial write to stdout.
	if _, err := io.WriteString(lro, h.stdoutString); err != nil {
		h.exitErr = err
		close(h.doneCh)
		h.logger.Printf("[ERR] mock_driver: failed to write to stdout: %v", err)
		return
	}

	for i := 0; i < h.stdoutRepeat; i++ {
		select {
		case <-h.doneCh:
			return
		case <-time.After(h.stdoutRepeatDur):
			if _, err := io.WriteString(lro, h.stdoutString); err != nil {
				h.exitErr = err
				close(h.doneCh)
				h.logger.Printf("[ERR] mock_driver: failed to write to stdout: %v", err)
				return
			}
		}
	}
}
