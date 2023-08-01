package testutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/lib/cgutil"
	"github.com/hashicorp/nomad/client/logmon"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	testing "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

type DriverHarness struct {
	drivers.DriverPlugin
	client *plugin.GRPCClient
	server *plugin.GRPCServer
	t      testing.T
	logger hclog.Logger
	impl   drivers.DriverPlugin
	cgroup string
}

func (h *DriverHarness) Impl() drivers.DriverPlugin {
	return h.impl
}
func NewDriverHarness(t testing.T, d drivers.DriverPlugin) *DriverHarness {
	logger := testlog.HCLogger(t).Named("driver_harness")
	pd := drivers.NewDriverPlugin(d, logger)

	client, server := plugin.TestPluginGRPCConn(t,
		map[string]plugin.Plugin{
			base.PluginTypeDriver: pd,
			base.PluginTypeBase:   &base.PluginBase{Impl: d},
			"logmon":              logmon.NewPlugin(logmon.NewLogMon(logger.Named("logmon"))),
		},
	)

	raw, err := client.Dispense(base.PluginTypeDriver)
	require.NoError(t, err, "failed to dispense plugin")

	dClient := raw.(drivers.DriverPlugin)
	return &DriverHarness{
		client:       client,
		server:       server,
		DriverPlugin: dClient,
		logger:       logger,
		t:            t,
		impl:         d,
	}
}

// setupCgroupV2 creates a v2 cgroup for the task, as if a Client were initialized
// and managing the cgroup as it normally would via the cpuset manager.
//
// Note that we are being lazy and trying to avoid importing cgutil because
// currently plugins/drivers/testutils is platform agnostic-ish.
//
// Some drivers (raw_exec) setup their own cgroup, while others (exec, java, docker)
// would otherwise depend on the Nomad cpuset manager (and docker daemon) to create
// one, which isn't available here in testing, and so we create one via the harness.
// Plumbing such metadata through to the harness is a mind bender, so we just always
// create the cgroup, but at least put it under 'testing.slice'.
//
// tl;dr raw_exec tests should ignore this cgroup.
func (h *DriverHarness) setupCgroupV2(allocID, task string) {
	if cgutil.UseV2 {
		h.cgroup = filepath.Join(cgutil.CgroupRoot, "testing.slice", cgutil.CgroupScope(allocID, task))
		h.logger.Trace("create cgroup for test", "parent", "testing.slice", "id", allocID, "task", task, "path", h.cgroup)
		if err := os.MkdirAll(h.cgroup, 0755); err != nil {
			panic(err)
		}
	}
}

func (h *DriverHarness) Kill() {
	_ = h.client.Close()
	h.server.Stop()
	h.cleanupCgroup()
}

// cleanupCgroup might cleanup a cgroup that may or may not be tricked by DriverHarness.
func (h *DriverHarness) cleanupCgroup() {
	// some [non-exec] tests don't bother with MkAllocDir which is what would create
	// the cgroup, but then do call Kill, so in that case skip the cgroup cleanup
	if cgutil.UseV2 && h.cgroup != "" {
		if err := os.Remove(h.cgroup); err != nil && !os.IsNotExist(err) {
			// in some cases the driver will cleanup the cgroup itself, in which
			// case we do not care about the cgroup not existing at cleanup time
			h.t.Fatalf("failed to cleanup cgroup: %v", err)
		}
	}
}

// MkAllocDir creates a temporary directory and allocdir structure.
// If enableLogs is set to true a logmon instance will be started to write logs
// to the LogDir of the task
// A cleanup func is returned and should be deferred so as to not leak dirs
// between tests.
func (h *DriverHarness) MkAllocDir(t *drivers.TaskConfig, enableLogs bool) func() {
	dir, err := os.MkdirTemp("", "nomad_driver_harness-")
	require.NoError(h.t, err)

	allocDir := allocdir.NewAllocDir(h.logger, dir, t.AllocID)
	require.NoError(h.t, allocDir.Build())

	t.AllocDir = allocDir.AllocDir

	taskDir := allocDir.NewTaskDir(t.Name)

	caps, err := h.Capabilities()
	require.NoError(h.t, err)

	fsi := caps.FSIsolation
	h.logger.Trace("FS isolation", "fsi", fsi)
	require.NoError(h.t, taskDir.Build(fsi == drivers.FSIsolationChroot, ci.TinyChroot))

	task := &structs.Task{
		Name: t.Name,
		Env:  t.Env,
	}

	// Create the mock allocation
	alloc := mock.Alloc()
	alloc.ID = t.AllocID
	if t.Resources != nil {
		alloc.AllocatedResources.Tasks[task.Name] = t.Resources.NomadResources
	}

	taskBuilder := taskenv.NewBuilder(mock.Node(), alloc, task, "global")
	SetEnvvars(taskBuilder, fsi, taskDir)

	taskEnv := taskBuilder.Build()
	if t.Env == nil {
		t.Env = taskEnv.Map()
	} else {
		for k, v := range taskEnv.Map() {
			if _, ok := t.Env[k]; !ok {
				t.Env[k] = v
			}
		}
	}

	// setup a v2 cgroup for test cases that assume one exists
	h.setupCgroupV2(alloc.ID, task.Name)

	//logmon
	if enableLogs {
		lm := logmon.NewLogMon(h.logger.Named("logmon"))
		if runtime.GOOS == "windows" {
			id := uuid.Generate()[:8]
			t.StdoutPath = fmt.Sprintf("//./pipe/%s-%s.stdout", t.Name, id)
			t.StderrPath = fmt.Sprintf("//./pipe/%s-%s.stderr", t.Name, id)
		} else {
			t.StdoutPath = filepath.Join(taskDir.LogDir, fmt.Sprintf(".%s.stdout.fifo", t.Name))
			t.StderrPath = filepath.Join(taskDir.LogDir, fmt.Sprintf(".%s.stderr.fifo", t.Name))
		}
		err = lm.Start(&logmon.LogConfig{
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
			lm.Stop()
			h.client.Close()
			allocDir.Destroy()
		}
	}

	return func() {
		h.client.Close()
		allocDir.Destroy()
		h.cleanupCgroup()
	}
}

// WaitUntilStarted will block until the task for the given ID is in the running
// state or the timeout is reached
func (h *DriverHarness) WaitUntilStarted(taskID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastState drivers.TaskState
	for {
		status, err := h.InspectTask(taskID)
		if err != nil {
			return err
		}
		if status.State == drivers.TaskStateRunning {
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
	TaskConfigSchemaF  func() (*hclspec.Spec, error)
	FingerprintF       func(context.Context) (<-chan *drivers.Fingerprint, error)
	CapabilitiesF      func() (*drivers.Capabilities, error)
	RecoverTaskF       func(*drivers.TaskHandle) error
	StartTaskF         func(*drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error)
	WaitTaskF          func(context.Context, string) (<-chan *drivers.ExitResult, error)
	StopTaskF          func(string, time.Duration, string) error
	DestroyTaskF       func(string, bool) error
	InspectTaskF       func(string) (*drivers.TaskStatus, error)
	TaskStatsF         func(context.Context, string, time.Duration) (<-chan *drivers.TaskResourceUsage, error)
	TaskEventsF        func(context.Context) (<-chan *drivers.TaskEvent, error)
	SignalTaskF        func(string, string) error
	ExecTaskF          func(string, []string, time.Duration) (*drivers.ExecTaskResult, error)
	ExecTaskStreamingF func(context.Context, string, *drivers.ExecOptions) (*drivers.ExitResult, error)
	MockNetworkManager
}

type MockNetworkManager struct {
	CreateNetworkF  func(string, *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error)
	DestroyNetworkF func(string, *drivers.NetworkIsolationSpec) error
}

func (m *MockNetworkManager) CreateNetwork(allocID string, req *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error) {
	return m.CreateNetworkF(allocID, req)
}
func (m *MockNetworkManager) DestroyNetwork(id string, spec *drivers.NetworkIsolationSpec) error {
	return m.DestroyNetworkF(id, spec)
}

func (d *MockDriver) TaskConfigSchema() (*hclspec.Spec, error) { return d.TaskConfigSchemaF() }
func (d *MockDriver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	return d.FingerprintF(ctx)
}
func (d *MockDriver) Capabilities() (*drivers.Capabilities, error) { return d.CapabilitiesF() }
func (d *MockDriver) RecoverTask(h *drivers.TaskHandle) error      { return d.RecoverTaskF(h) }
func (d *MockDriver) StartTask(c *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	return d.StartTaskF(c)
}
func (d *MockDriver) WaitTask(ctx context.Context, id string) (<-chan *drivers.ExitResult, error) {
	return d.WaitTaskF(ctx, id)
}
func (d *MockDriver) StopTask(taskID string, timeout time.Duration, signal string) error {
	return d.StopTaskF(taskID, timeout, signal)
}
func (d *MockDriver) DestroyTask(taskID string, force bool) error {
	return d.DestroyTaskF(taskID, force)
}
func (d *MockDriver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	return d.InspectTaskF(taskID)
}
func (d *MockDriver) TaskStats(ctx context.Context, taskID string, i time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	return d.TaskStatsF(ctx, taskID, i)
}
func (d *MockDriver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.TaskEventsF(ctx)
}
func (d *MockDriver) SignalTask(taskID string, signal string) error {
	return d.SignalTaskF(taskID, signal)
}
func (d *MockDriver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	return d.ExecTaskF(taskID, cmd, timeout)
}

func (d *MockDriver) ExecTaskStreaming(ctx context.Context, taskID string, execOpts *drivers.ExecOptions) (*drivers.ExitResult, error) {
	return d.ExecTaskStreamingF(ctx, taskID, execOpts)
}

// SetEnvvars sets path and host env vars depending on the FS isolation used.
func SetEnvvars(envBuilder *taskenv.Builder, fsi drivers.FSIsolation, taskDir *allocdir.TaskDir) {

	envBuilder.SetClientTaskRoot(taskDir.Dir)
	envBuilder.SetClientSharedAllocDir(taskDir.SharedAllocDir)
	envBuilder.SetClientTaskLocalDir(taskDir.LocalDir)
	envBuilder.SetClientTaskSecretsDir(taskDir.SecretsDir)

	// Set driver-specific environment variables
	switch fsi {
	case drivers.FSIsolationNone:
		// Use host paths
		envBuilder.SetAllocDir(taskDir.SharedAllocDir)
		envBuilder.SetTaskLocalDir(taskDir.LocalDir)
		envBuilder.SetSecretsDir(taskDir.SecretsDir)
	default:
		// filesystem isolation; use container paths
		envBuilder.SetAllocDir(allocdir.SharedAllocContainerPath)
		envBuilder.SetTaskLocalDir(allocdir.TaskLocalContainerPath)
		envBuilder.SetSecretsDir(allocdir.TaskSecretsContainerPath)
	}

	// Set the host environment variables for non-image based drivers
	if fsi != drivers.FSIsolationImage {
		envBuilder.SetHostEnvvars([]string{"env.denylist"})
	}
}
