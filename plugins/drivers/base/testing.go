package base

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	plugin "github.com/hashicorp/go-plugin"
)

type DriverHarness struct {
	DriverClient
	client *plugin.GRPCClient
	server *plugin.GRPCServer
	t      testing.T
}

func NewDriverHarness(t testing.T, d Driver) *DriverHarness {
	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		DriverGoPlugin: &DriverPlugin{impl: d},
	})

	raw, err := client.Dispense(DriverGoPlugin)
	if err != nil {
		t.Fatalf("err dispensing plugin: %v", err)
	}

	dClient := raw.(DriverClient)
	h := &DriverHarness{
		client:       client,
		server:       server,
		DriverClient: dClient,
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
	RecoverTaskF func(*TaskHandle) error
	StartTaskF   func(*TaskConfig) (*TaskHandle, error)
	WaitTaskF    func(string) chan *TaskResult
}

func (d *MockDriver) Fingerprint() *Fingerprint                                          { return nil }
func (d *MockDriver) Capabilities() *Capabilities                                        { return nil }
func (d *MockDriver) RecoverTask(h *TaskHandle) error                                    { return d.RecoverTaskF(h) }
func (d *MockDriver) StartTask(c *TaskConfig) (*TaskHandle, error)                       { return d.StartTaskF(c) }
func (d *MockDriver) WaitTask(id string) chan *TaskResult                                { return d.WaitTaskF(id) }
func (d *MockDriver) StopTask(taskID string, timeout time.Duration, signal string) error { return nil }
func (d *MockDriver) DestroyTask(taskID string)                                          {}
func (d *MockDriver) ListTasks(*ListTasksQuery) ([]*TaskSummary, error)                  { return nil, nil }
func (d *MockDriver) InspectTask(taskID string) (*TaskStatus, error)                     { return nil, nil }
func (d *MockDriver) TaskStats(taskID string) (*TaskStats, error)                        { return nil, nil }
