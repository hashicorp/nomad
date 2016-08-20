package driver

import (
	"errors"
	"log"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/nomad/client/config"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// MockDriverConfig is the driver configuration for the MockDriver
type MockDriverConfig struct {

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
}

// MockDriver is a driver which is used for testing purposes
type MockDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter
}

// NewMockDriver is a factory method which returns a new Mock Driver
func NewMockDriver(ctx *DriverContext) Driver {
	return &MockDriver{DriverContext: *ctx}
}

// Start starts the mock driver
func (m *MockDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
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

	h := mockDriverHandle{
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
	go h.run()
	return &h, nil
}

// TODO implement Open when we need it.
// Open re-connects the driver to the running task
func (m *MockDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	return nil, nil
}

// TODO implement Open when we need it.
// Validate validates the mock driver configuration
func (m *MockDriver) Validate(map[string]interface{}) error {
	return nil
}

// TODO implement Open when we need it.
// Fingerprint fingerprints a node and returns if MockDriver is enabled
func (m *MockDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	return true, nil
}

// MockDriverHandle is a driver handler which supervises a mock task
type mockDriverHandle struct {
	runFor      time.Duration
	killAfter   time.Duration
	killTimeout time.Duration
	exitCode    int
	exitSignal  int
	exitErr     error
	logger      *log.Logger
	waitCh      chan *dstructs.WaitResult
	doneCh      chan struct{}
}

// TODO Implement when we need it.
func (h *mockDriverHandle) ID() string {
	return ""
}

// TODO Implement when we need it.
func (h *mockDriverHandle) WaitCh() chan *dstructs.WaitResult {
	return h.waitCh
}

// TODO Implement when we need it.
func (h *mockDriverHandle) Update(task *structs.Task) error {
	return nil
}

// Kill kills a mock task
func (h *mockDriverHandle) Kill() error {
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(h.killTimeout):
		close(h.doneCh)
		return nil
	}
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
			h.waitCh <- dstructs.NewWaitResult(h.exitCode, h.exitSignal, h.exitErr)
			return
		}
	}
}
