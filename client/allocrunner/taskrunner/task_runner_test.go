package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/config"
	consulapi "github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/devicemanager"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	cstate "github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/client/vaultclient"
	mockdriver "github.com/hashicorp/nomad/drivers/mock"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockTaskStateUpdater struct {
	ch chan struct{}
}

func NewMockTaskStateUpdater() *MockTaskStateUpdater {
	return &MockTaskStateUpdater{
		ch: make(chan struct{}, 1),
	}
}

func (m *MockTaskStateUpdater) TaskStateUpdated() {
	select {
	case m.ch <- struct{}{}:
	default:
	}
}

// testTaskRunnerConfig returns a taskrunner.Config for the given alloc+task
// plus a cleanup func.
func testTaskRunnerConfig(t *testing.T, alloc *structs.Allocation, taskName string) (*Config, func()) {
	logger := testlog.HCLogger(t)
	clientConf, cleanup := config.TestClientConfig(t)

	// Find the task
	var thisTask *structs.Task
	for _, tg := range alloc.Job.TaskGroups {
		for _, task := range tg.Tasks {
			if task.Name == taskName {
				if thisTask != nil {
					cleanup()
					t.Fatalf("multiple tasks named %q; cannot use this helper", taskName)
				}
				thisTask = task
			}
		}
	}
	if thisTask == nil {
		cleanup()
		t.Fatalf("could not find task %q", taskName)
	}

	// Create the alloc dir + task dir
	allocPath := filepath.Join(clientConf.AllocDir, alloc.ID)
	allocDir := allocdir.NewAllocDir(logger, allocPath)
	if err := allocDir.Build(); err != nil {
		cleanup()
		t.Fatalf("error building alloc dir: %v", err)
	}
	taskDir := allocDir.NewTaskDir(taskName)

	trCleanup := func() {
		if err := allocDir.Destroy(); err != nil {
			t.Logf("error destroying alloc dir: %v", err)
		}
		cleanup()
	}

	conf := &Config{
		Alloc:         alloc,
		ClientConfig:  clientConf,
		Consul:        consulapi.NewMockConsulServiceClient(t, logger),
		Task:          thisTask,
		TaskDir:       taskDir,
		Logger:        clientConf.Logger,
		Vault:         vaultclient.NewMockVaultClient(),
		StateDB:       cstate.NoopDB{},
		StateUpdater:  NewMockTaskStateUpdater(),
		DeviceManager: devicemanager.NoopMockManager(),
		DriverManager: drivermanager.TestDriverManager(t),
	}
	return conf, trCleanup
}

// TestTaskRunner_Restore asserts restoring a running task does not rerun the
// task.
func TestTaskRunner_Restore_Running(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	alloc := mock.BatchAlloc()
	alloc.Job.TaskGroups[0].Count = 1
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "2s",
	}
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	conf.StateDB = cstate.NewMemDB() // "persist" state between task runners
	defer cleanup()

	// Run the first TaskRunner
	origTR, err := NewTaskRunner(conf)
	require.NoError(err)
	go origTR.Run()
	defer origTR.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait for it to be running
	testutil.WaitForResult(func() (bool, error) {
		ts := origTR.TaskState()
		return ts.State == structs.TaskStateRunning, fmt.Errorf("%v", ts.State)
	}, func(err error) {
		t.Fatalf("expected running; got: %v", err)
	})

	// Cause TR to exit without shutting down task
	origTR.Shutdown()

	// Start a new TaskRunner and make sure it does not rerun the task
	newTR, err := NewTaskRunner(conf)
	require.NoError(err)

	// Do the Restore
	require.NoError(newTR.Restore())

	go newTR.Run()
	defer newTR.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait for new task runner to exit when the process does
	<-newTR.WaitCh()

	// Assert that the process was only started once
	started := 0
	state := newTR.TaskState()
	require.Equal(structs.TaskStateDead, state.State)
	for _, ev := range state.Events {
		if ev.Type == structs.TaskStarted {
			started++
		}
	}
	assert.Equal(t, 1, started)
}

// TestTaskRunner_TaskEnv asserts driver configurations are interpolated.
func TestTaskRunner_TaskEnv(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	alloc := mock.BatchAlloc()
	alloc.Job.TaskGroups[0].Meta = map[string]string{
		"common_user": "somebody",
	}
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Meta = map[string]string{
		"foo": "bar",
	}

	// Use interpolation from both node attributes and meta vars
	task.Config = map[string]interface{}{
		"run_for":       "1ms",
		"stdout_string": `${node.region} ${NOMAD_META_foo} ${NOMAD_META_common_user}`,
	}

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()

	// Run the first TaskRunner
	tr, err := NewTaskRunner(conf)
	require.NoError(err)
	go tr.Run()
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait for task to complete
	select {
	case <-tr.WaitCh():
	case <-time.After(3 * time.Second):
	}

	// Get the mock driver plugin
	driverPlugin, err := conf.DriverManager.Dispense(mockdriver.PluginID.Name)
	require.NoError(err)
	mockDriver := driverPlugin.(*mockdriver.Driver)

	// Assert its config has been properly interpolated
	driverCfg, mockCfg := mockDriver.GetTaskConfig()
	require.NotNil(driverCfg)
	require.NotNil(mockCfg)
	assert.Equal(t, "global bar somebody", mockCfg.StdoutString)
}

// Test that devices get sent to the driver
func TestTaskRunner_DevicePropogation(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create a mock alloc that has a gpu
	alloc := mock.BatchAlloc()
	alloc.Job.TaskGroups[0].Count = 1
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "100ms",
	}
	tRes := alloc.AllocatedResources.Tasks[task.Name]
	tRes.Devices = append(tRes.Devices, &structs.AllocatedDeviceResource{Type: "mock"})

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	conf.StateDB = cstate.NewMemDB() // "persist" state between task runners
	defer cleanup()

	// Setup the devicemanager
	dm, ok := conf.DeviceManager.(*devicemanager.MockManager)
	require.True(ok)

	dm.ReserveF = func(d *structs.AllocatedDeviceResource) (*device.ContainerReservation, error) {
		res := &device.ContainerReservation{
			Envs: map[string]string{
				"ABC": "123",
			},
			Mounts: []*device.Mount{
				{
					ReadOnly: true,
					TaskPath: "foo",
					HostPath: "bar",
				},
			},
			Devices: []*device.DeviceSpec{
				{
					TaskPath:    "foo",
					HostPath:    "bar",
					CgroupPerms: "123",
				},
			},
		}
		return res, nil
	}

	// Run the TaskRunner
	tr, err := NewTaskRunner(conf)
	require.NoError(err)
	go tr.Run()
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait for task to complete
	select {
	case <-tr.WaitCh():
	case <-time.After(3 * time.Second):
	}

	// Get the mock driver plugin
	driverPlugin, err := conf.DriverManager.Dispense(mockdriver.PluginID.Name)
	require.NoError(err)
	mockDriver := driverPlugin.(*mockdriver.Driver)

	// Assert its config has been properly interpolated
	driverCfg, _ := mockDriver.GetTaskConfig()
	require.NotNil(driverCfg)
	require.Len(driverCfg.Devices, 1)
	require.Equal(driverCfg.Devices[0].Permissions, "123")
	require.Len(driverCfg.Mounts, 1)
	require.Equal(driverCfg.Mounts[0].TaskPath, "foo")
	require.Contains(driverCfg.Env, "ABC")
}

// mockEnvHook is a test hook that sets an env var and done=true. It fails if
// it's called more than once.
type mockEnvHook struct {
	called int
}

func (*mockEnvHook) Name() string {
	return "mock_env_hook"
}

func (h *mockEnvHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	h.called++

	resp.Done = true
	resp.Env = map[string]string{
		"mock_hook": "1",
	}

	return nil
}

// TestTaskRunner_Restore_HookEnv asserts that re-running prestart hooks with
// hook environments set restores the environment without re-running done
// hooks.
func TestTaskRunner_Restore_HookEnv(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	conf.StateDB = cstate.NewMemDB() // "persist" state between prestart calls
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(err)

	// Override the default hooks to only run the mock hook
	mockHook := &mockEnvHook{}
	tr.runnerHooks = []interfaces.TaskHook{mockHook}

	// Manually run prestart hooks
	require.NoError(tr.prestart())

	// Assert env was called
	require.Equal(1, mockHook.called)

	// Re-running prestart hooks should *not* call done mock hook
	require.NoError(tr.prestart())

	// Assert env was called
	require.Equal(1, mockHook.called)

	// Assert the env is still set
	env := tr.envBuilder.Build().All()
	require.Contains(env, "mock_hook")
	require.Equal("1", env["mock_hook"])
}

// This test asserts that we can recover from an "external" plugin exiting by
// retrieving a new instance of the driver and recovering the task.
func TestTaskRunner_RecoverFromDriverExiting(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create an allocation using the mock driver that exits simulating the
	// driver crashing. We can then test that the task runner recovers from this
	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"plugin_exit_after": "1s",
		"run_for":           "5s",
	}

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	conf.StateDB = cstate.NewMemDB() // "persist" state between prestart calls
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(err)

	start := time.Now()
	go tr.Run()
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait for the task to be running
	testWaitForTaskToStart(t, tr)

	// Get the task ID
	tr.stateLock.RLock()
	l := tr.localState.TaskHandle
	require.NotNil(l)
	require.NotNil(l.Config)
	require.NotEmpty(l.Config.ID)
	id := l.Config.ID
	tr.stateLock.RUnlock()

	// Get the mock driver plugin
	driverPlugin, err := conf.DriverManager.Dispense(mockdriver.PluginID.Name)
	require.NoError(err)
	mockDriver := driverPlugin.(*mockdriver.Driver)

	// Wait for the task to start
	testutil.WaitForResult(func() (bool, error) {
		// Get the handle and check that it was recovered
		handle := mockDriver.GetHandle(id)
		if handle == nil {
			return false, fmt.Errorf("nil handle")
		}
		if !handle.Recovered {
			return false, fmt.Errorf("handle not recovered")
		}
		return true, nil
	}, func(err error) {
		t.Fatal(err.Error())
	})

	// Wait for task to complete
	select {
	case <-tr.WaitCh():
	case <-time.After(10 * time.Second):
	}

	// Ensure that we actually let the task complete
	require.True(time.Now().Sub(start) > 5*time.Second)

	// Check it finished successfully
	state := tr.TaskState()
	require.True(state.Successful())
}

// testWaitForTaskToStart waits for the task to or fails the test
func testWaitForTaskToStart(t *testing.T, tr *TaskRunner) {
	// Wait for the task to start
	testutil.WaitForResult(func() (bool, error) {
		tr.stateLock.RLock()
		started := !tr.state.StartedAt.IsZero()
		tr.stateLock.RUnlock()

		return started, nil
	}, func(err error) {
		t.Fatalf("not started")
	})
}
