//+build nomad_test

package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/nomad/client/config"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Add the mock driver to the list of builtin drivers
func init() {
	BuiltinDrivers["mock_driver"] = NewMockDriver
}

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
}

// MockDriver is a driver which is used for testing purposes
type MockDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter

	cleanupFailNum int
}

// NewMockDriver is a factory method which returns a new Mock Driver
func NewMockDriver(ctx *DriverContext) Driver {
	return &MockDriver{DriverContext: *ctx}
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

	h := mockDriverHandle{
		taskName:    task.Name,
		runFor:      driverConfig.RunFor,
		killAfter:   driverConfig.KillAfter,
		killTimeout: task.KillTimeout,
		exitCode:    driverConfig.ExitCode,
		exitSignal:  driverConfig.ExitSignal,
		logger:      m.logger,
		doneCh:      make(chan struct{}),
		waitCh:      make(chan *dstructs.WaitResult, 1),
	}
	if driverConfig.ExitErrMsg != "" {
		h.exitErr = errors.New(driverConfig.ExitErrMsg)
	}
	if driverConfig.SignalErr != "" {
		h.signalErr = fmt.Errorf(driverConfig.SignalErr)
	}
	m.logger.Printf("[DEBUG] driver.mock: starting task %q", task.Name)
	go h.run()
	return &StartResponse{Handle: &h}, nil
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
func (m *MockDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	node.Attributes["driver.mock_driver"] = "1"
	return true, nil
}

// MockDriverHandle is a driver handler which supervises a mock task
type mockDriverHandle struct {
	taskName    string
	runFor      time.Duration
	killAfter   time.Duration
	killTimeout time.Duration
	exitCode    int
	exitSignal  int
	exitErr     error
	signalErr   error
	logger      *log.Logger
	waitCh      chan *dstructs.WaitResult
	doneCh      chan struct{}
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
		close(h.doneCh)
	case <-time.After(h.killTimeout):
		h.logger.Printf("[DEBUG] driver.mock: terminating task %q", h.taskName)
		close(h.doneCh)
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
	timer := time.NewTimer(h.runFor)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			close(h.doneCh)
		case <-h.doneCh:
			h.logger.Printf("[DEBUG] driver.mock: finished running task %q", h.taskName)
			h.waitCh <- dstructs.NewWaitResult(h.exitCode, h.exitSignal, h.exitErr)
			return
		}
	}
}
