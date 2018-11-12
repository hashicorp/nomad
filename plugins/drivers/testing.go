package drivers

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/logmon"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers/utils"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

type DriverHarness struct {
	DriverPlugin
	client *plugin.GRPCClient
	server *plugin.GRPCServer
	t      testing.T
	lm     logmon.LogMon
	logger hclog.Logger
	impl   DriverPlugin
}

func (d *DriverHarness) Impl() DriverPlugin {
	return d.impl
}
func NewDriverHarness(t testing.T, d DriverPlugin) *DriverHarness {
	logger := testlog.HCLogger(t).Named("driver_harness")
	client, server := plugin.TestPluginGRPCConn(t,
		map[string]plugin.Plugin{
			base.PluginTypeDriver: &PluginDriver{
				impl:   d,
				logger: logger.Named("driver_plugin"),
			},
			base.PluginTypeBase: &base.PluginBase{Impl: d},
			"logmon":            logmon.NewPlugin(logmon.NewLogMon(logger.Named("logmon"))),
		},
	)

	raw, err := client.Dispense(base.PluginTypeDriver)
	if err != nil {
		t.Fatalf("err dispensing plugin: %v", err)
	}

	dClient := raw.(DriverPlugin)
	h := &DriverHarness{
		client:       client,
		server:       server,
		DriverPlugin: dClient,
		logger:       logger,
		t:            t,
		impl:         d,
	}

	raw, err = client.Dispense("logmon")
	if err != nil {
		t.Fatalf("err dispensing plugin: %v", err)
	}

	h.lm = raw.(logmon.LogMon)
	return h
}

func (h *DriverHarness) Kill() {
	h.client.Close()
	h.server.Stop()
}

// MkAllocDir creates a tempory directory and allocdir structure.
// If enableLogs is set to true a logmon instance will be started to write logs
// to the LogDir of the task
// A cleanup func is returned and should be defered so as to not leak dirs
// between tests.
func (h *DriverHarness) MkAllocDir(t *TaskConfig, enableLogs bool) func() {
	dir, err := ioutil.TempDir("", "nomad_driver_harness-")
	require.NoError(h.t, err)
	t.AllocDir = dir

	allocDir := allocdir.NewAllocDir(h.logger, dir)
	require.NoError(h.t, allocDir.Build())
	taskDir := allocDir.NewTaskDir(t.Name)

	caps, err := h.Capabilities()
	require.NoError(h.t, err)

	var entries map[string]string
	fsi := caps.FSIsolation
	if fsi == cstructs.FSIsolationChroot {
		entries = config.DefaultChrootEnv
	}
	require.NoError(h.t, taskDir.Build(false, entries, fsi))

	task := &structs.Task{
		Name:      t.Name,
		Env:       t.Env,
		Resources: t.Resources.NomadResources,
	}
	taskBuilder := env.NewBuilder(mock.Node(), mock.Alloc(), task, "global")
	utils.SetEnvvars(taskBuilder, fsi, taskDir, config.DefaultConfig())

	taskEnv := taskBuilder.Build()
	if t.Env == nil {
		t.Env = taskEnv.All()
	} else {
		for k, v := range taskEnv.All() {
			if _, ok := t.Env[k]; !ok {
				t.Env[k] = v
			}
		}
	}

	//logmon
	if enableLogs {
		if runtime.GOOS == "windows" {
			id := uuid.Generate()[:8]
			t.StdoutPath = fmt.Sprintf("//./pipe/%s-%s.stdout", t.Name, id)
			t.StderrPath = fmt.Sprintf("//./pipe/%s-%s.stderr", t.Name, id)
		} else {
			t.StdoutPath = filepath.Join(taskDir.LogDir, fmt.Sprintf(".%s.stdout.fifo", t.Name))
			t.StderrPath = filepath.Join(taskDir.LogDir, fmt.Sprintf(".%s.stderr.fifo", t.Name))
		}
		err = h.lm.Start(&logmon.LogConfig{
			LogDir:        taskDir.LogDir,
			StdoutLogFile: fmt.Sprintf("%s.stdout", t.Name),
			StderrLogFile: fmt.Sprintf("%s.stderr", t.Name),
			StdoutFifo:    t.StdoutPath,
			StderrFifo:    t.StderrPath,
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		})
		require.NoError(h.t, err)

		return func() {
			h.lm.Stop()
			allocDir.Destroy()
		}
	}

	return func() {
		if h.lm != nil {
			h.lm.Stop()
		}
		allocDir.Destroy()
	}
}

// WaitUntilStarted will block until the task for the given ID is in the running
// state or the timeout is reached
func (h *DriverHarness) WaitUntilStarted(taskID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastState TaskState
	for {
		status, err := h.InspectTask(taskID)
		if err != nil {
			return err
		}
		if status.State == TaskStateRunning {
			return nil
		}
		lastState = status.State
		if time.Now().After(deadline) {
			return fmt.Errorf("task never transitioned to running, currently '%s'", lastState)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// MockDriver is used for testing.
// Each function can be set as a closure to make assertions about how data
// is passed through the base plugin layer.
type MockDriver struct {
	base.MockPlugin
	TaskConfigSchemaF func() (*hclspec.Spec, error)
	FingerprintF      func(context.Context) (<-chan *Fingerprint, error)
	CapabilitiesF     func() (*Capabilities, error)
	RecoverTaskF      func(*TaskHandle) error
	StartTaskF        func(*TaskConfig) (*TaskHandle, *cstructs.DriverNetwork, error)
	WaitTaskF         func(context.Context, string) (<-chan *ExitResult, error)
	StopTaskF         func(string, time.Duration, string) error
	DestroyTaskF      func(string, bool) error
	InspectTaskF      func(string) (*TaskStatus, error)
	TaskStatsF        func(string) (*cstructs.TaskResourceUsage, error)
	TaskEventsF       func(context.Context) (<-chan *TaskEvent, error)
	SignalTaskF       func(string, string) error
	ExecTaskF         func(string, []string, time.Duration) (*ExecTaskResult, error)
}

func (d *MockDriver) TaskConfigSchema() (*hclspec.Spec, error) { return d.TaskConfigSchemaF() }
func (d *MockDriver) Fingerprint(ctx context.Context) (<-chan *Fingerprint, error) {
	return d.FingerprintF(ctx)
}
func (d *MockDriver) Capabilities() (*Capabilities, error) { return d.CapabilitiesF() }
func (d *MockDriver) RecoverTask(h *TaskHandle) error      { return d.RecoverTaskF(h) }
func (d *MockDriver) StartTask(c *TaskConfig) (*TaskHandle, *cstructs.DriverNetwork, error) {
	return d.StartTaskF(c)
}
func (d *MockDriver) WaitTask(ctx context.Context, id string) (<-chan *ExitResult, error) {
	return d.WaitTaskF(ctx, id)
}
func (d *MockDriver) StopTask(taskID string, timeout time.Duration, signal string) error {
	return d.StopTaskF(taskID, timeout, signal)
}
func (d *MockDriver) DestroyTask(taskID string, force bool) error {
	return d.DestroyTaskF(taskID, force)
}
func (d *MockDriver) InspectTask(taskID string) (*TaskStatus, error) { return d.InspectTaskF(taskID) }
func (d *MockDriver) TaskStats(taskID string) (*cstructs.TaskResourceUsage, error) {
	return d.TaskStats(taskID)
}
func (d *MockDriver) TaskEvents(ctx context.Context) (<-chan *TaskEvent, error) {
	return d.TaskEventsF(ctx)
}
func (d *MockDriver) SignalTask(taskID string, signal string) error {
	return d.SignalTask(taskID, signal)
}
func (d *MockDriver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*ExecTaskResult, error) {
	return d.ExecTaskF(taskID, cmd, timeout)
}
