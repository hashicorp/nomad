package base

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

type DriverHarness struct {
	DriverPlugin
	client *plugin.GRPCClient
	server *plugin.GRPCServer
	t      testing.T
}

func NewDriverHarness(t testing.T, d DriverPlugin) *DriverHarness {
	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		DriverGoPlugin: &PluginDriver{
			impl:   d,
			logger: testlog.HCLogger(t),
		},
	})

	raw, err := client.Dispense(DriverGoPlugin)
	if err != nil {
		t.Fatalf("err dispensing plugin: %v", err)
	}

	dClient := raw.(DriverPlugin)
	h := &DriverHarness{
		client:       client,
		server:       server,
		DriverPlugin: dClient,
	}

	return h
}

func (h *DriverHarness) Kill() {
	h.client.Close()
	h.server.Stop()
}

// MkAllocDir creates a tempory directory and allocdir structure.
// A cleanup func is returned and should be defered so as to not leak dirs
// between tests.
func (h *DriverHarness) MkAllocDir(t *TaskConfig) func() {
	allocDir, err := ioutil.TempDir("", "nomad_driver_harness-")
	require.NoError(h.t, err)
	os.Mkdir(filepath.Join(allocDir, t.Name), os.ModePerm)
	os.MkdirAll(filepath.Join(allocDir, "alloc/logs"), os.ModePerm)
	t.AllocDir = allocDir
	return func() { os.RemoveAll(allocDir) }
}

// MockDriver is used for testing.
// Each function can be set as a closure to make assertions about how data
// is passed through the base plugin layer.
type MockDriver struct {
	base.MockPlugin
	TaskConfigSchemaF func() (*hclspec.Spec, error)
	FingerprintF      func() (chan *Fingerprint, error)
	CapabilitiesF     func() (*Capabilities, error)
	RecoverTaskF      func(*TaskHandle) error
	StartTaskF        func(*TaskConfig) (*TaskHandle, error)
	WaitTaskF         func(context.Context, string) chan *ExitResult
	StopTaskF         func(string, time.Duration, string) error
	DestroyTaskF      func(string, bool) error
	InspectTaskF      func(string) (*TaskStatus, error)
	TaskStatsF        func(string) (*TaskStats, error)
	TaskEventsF       func() (chan *TaskEvent, error)
	SignalTaskF       func(string, string) error
	ExecTaskF         func(string, []string, time.Duration) (*ExecTaskResult, error)
}

func (d *MockDriver) TaskConfigSchema() (*hclspec.Spec, error)     { return d.TaskConfigSchemaF() }
func (d *MockDriver) Fingerprint() (chan *Fingerprint, error)      { return d.FingerprintF() }
func (d *MockDriver) Capabilities() (*Capabilities, error)         { return d.CapabilitiesF() }
func (d *MockDriver) RecoverTask(h *TaskHandle) error              { return d.RecoverTaskF(h) }
func (d *MockDriver) StartTask(c *TaskConfig) (*TaskHandle, error) { return d.StartTaskF(c) }
func (d *MockDriver) WaitTask(ctx context.Context, id string) chan *ExitResult {
	return d.WaitTaskF(ctx, id)
}
func (d *MockDriver) StopTask(taskID string, timeout time.Duration, signal string) error {
	return d.StopTaskF(taskID, timeout, signal)
}
func (d *MockDriver) DestroyTask(taskID string, force bool) error {
	return d.DestroyTaskF(taskID, force)
}
func (d *MockDriver) InspectTask(taskID string) (*TaskStatus, error) { return d.InspectTaskF(taskID) }
func (d *MockDriver) TaskStats(taskID string) (*TaskStats, error)    { return d.TaskStats(taskID) }
func (d *MockDriver) TaskEvents() (chan *TaskEvent, error)           { return d.TaskEventsF() }
func (d *MockDriver) SignalTask(taskID string, signal string) error {
	return d.SignalTask(taskID, signal)
}
func (d *MockDriver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*ExecTaskResult, error) {
	return d.ExecTaskF(taskID, cmd, timeout)
}
