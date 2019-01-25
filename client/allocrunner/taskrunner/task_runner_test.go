package taskrunner

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/consul"
	consulapi "github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/devicemanager"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	cstate "github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/client/vaultclient"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	mockdriver "github.com/hashicorp/nomad/drivers/mock"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
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
	testWaitForTaskToStart(t, origTR)

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

func TestTaskRunner_TaskConfig(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"

	//// Use interpolation from both node attributes and meta vars
	//task.Config = map[string]interface{}{
	//	"run_for": "1ms",
	//}

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
	assert.Equal(t, alloc.Job.Name, driverCfg.JobName)
	assert.Equal(t, alloc.TaskGroup, driverCfg.TaskGroupName)
	assert.Equal(t, alloc.Job.TaskGroups[0].Tasks[0].Name, driverCfg.Name)
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

// TestTaskRunner_ShutdownDelay asserts services are removed from Consul
// ${shutdown_delay} seconds before killing the process.
func TestTaskRunner_ShutdownDelay(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Services[0].Tags = []string{"tag1"}
	task.Services = task.Services[:1] // only need 1 for this test
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1000s",
	}

	// No shutdown escape hatch for this delay, so don't set it too high
	task.ShutdownDelay = 500 * time.Duration(testutil.TestMultiplier()) * time.Millisecond

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()

	mockConsul := conf.Consul.(*consul.MockConsulServiceClient)

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	go tr.Run()
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait for the task to start
	testWaitForTaskToStart(t, tr)

	testutil.WaitForResult(func() (bool, error) {
		ops := mockConsul.GetOps()
		if n := len(ops); n != 1 {
			return false, fmt.Errorf("expected 1 consul operation. Found %d", n)
		}
		return ops[0].Op == "add", fmt.Errorf("consul operation was not a registration: %#v", ops[0])
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Asynchronously kill task
	killSent := time.Now()
	killed := make(chan struct{})
	go func() {
		defer close(killed)
		assert.NoError(t, tr.Kill(context.Background(), structs.NewTaskEvent("test")))
	}()

	// Wait for *2* deregistration calls (due to needing to remove both
	// canary tag variants)
WAIT:
	for {
		ops := mockConsul.GetOps()
		switch n := len(ops); n {
		case 1, 2:
			// Waiting for both deregistration calls
		case 3:
			require.Equalf(t, "remove", ops[1].Op, "expected deregistration but found: %#v", ops[1])
			require.Equalf(t, "remove", ops[2].Op, "expected deregistration but found: %#v", ops[2])
			break WAIT
		default:
			// ?!
			t.Fatalf("unexpected number of consul operations: %d\n%s", n, pretty.Sprint(ops))

		}

		select {
		case <-killed:
			t.Fatal("killed while service still registered")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Wait for actual exit
	select {
	case <-tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	<-killed
	killDur := time.Now().Sub(killSent)
	if killDur < task.ShutdownDelay {
		t.Fatalf("task killed before shutdown_delay (killed_after: %s; shutdown_delay: %s",
			killDur, task.ShutdownDelay,
		)
	}
}

// TestTaskRunner_Dispatch_Payload asserts that a dispatch job runs and the
// payload was written to disk.
func TestTaskRunner_Dispatch_Payload(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1s",
	}

	fileName := "test"
	task.DispatchPayload = &structs.DispatchPayloadConfig{
		File: fileName,
	}
	alloc.Job.ParameterizedJob = &structs.ParameterizedJobConfig{}

	// Add a payload (they're snappy encoded bytes)
	expected := []byte("hello world")
	compressed := snappy.Encode(nil, expected)
	alloc.Job.Payload = compressed

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	go tr.Run()
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait for it to finish
	testutil.WaitForResult(func() (bool, error) {
		ts := tr.TaskState()
		return ts.State == structs.TaskStateDead, fmt.Errorf("%v", ts.State)
	}, func(err error) {
		require.NoError(t, err)
	})

	// Should have exited successfully
	ts := tr.TaskState()
	require.False(t, ts.Failed)
	require.Zero(t, ts.Restarts)

	// Check that the file was written to disk properly
	payloadPath := filepath.Join(tr.taskDir.LocalDir, fileName)
	data, err := ioutil.ReadFile(payloadPath)
	require.NoError(t, err)
	require.Equal(t, expected, data)
}

// TestTaskRunner_SignalFailure asserts that signal errors are properly
// propagated from the driver to TaskRunner.
func TestTaskRunner_SignalFailure(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	errMsg := "test forcing failure"
	task.Config = map[string]interface{}{
		"run_for":      "10m",
		"signal_error": errMsg,
	}

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	go tr.Run()
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	testWaitForTaskToStart(t, tr)

	require.EqualError(t, tr.Signal(&structs.TaskEvent{}, "SIGINT"), errMsg)
}

// TestTaskRunner_RestartTask asserts that restarting a task works and emits a
// Restarting event.
func TestTaskRunner_RestartTask(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10m",
	}

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	go tr.Run()
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	testWaitForTaskToStart(t, tr)

	// Restart task. Send a RestartSignal event like check watcher. Restart
	// handler emits the Restarting event.
	event := structs.NewTaskEvent(structs.TaskRestartSignal).SetRestartReason("test")
	const fail = false
	tr.Restart(context.Background(), event.Copy(), fail)

	// Wait for it to restart and be running again
	testutil.WaitForResult(func() (bool, error) {
		ts := tr.TaskState()
		if ts.Restarts != 1 {
			return false, fmt.Errorf("expected 1 restart but found %d\nevents: %s",
				ts.Restarts, pretty.Sprint(ts.Events))
		}
		if ts.State != structs.TaskStateRunning {
			return false, fmt.Errorf("expected running but received %s", ts.State)
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// Assert the expected Restarting event was emitted
	found := false
	events := tr.TaskState().Events
	for _, e := range events {
		if e.Type == structs.TaskRestartSignal {
			found = true
			require.Equal(t, event.Time, e.Time)
			require.Equal(t, event.RestartReason, e.RestartReason)
			require.Contains(t, e.DisplayMessage, event.RestartReason)
		}
	}
	require.True(t, found, "restarting task event not found", pretty.Sprint(events))
}

// TestTaskRunner_CheckWatcher_Restart asserts that when enabled an unhealthy
// Consul check will cause a task to restart following restart policy rules.
func TestTaskRunner_CheckWatcher_Restart(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()

	// Make the restart policy fail within this test
	tg := alloc.Job.TaskGroups[0]
	tg.RestartPolicy.Attempts = 2
	tg.RestartPolicy.Interval = 1 * time.Minute
	tg.RestartPolicy.Delay = 10 * time.Millisecond
	tg.RestartPolicy.Mode = structs.RestartPolicyModeFail

	task := tg.Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10m",
	}

	// Make the task register a check that fails
	task.Services[0].Checks[0] = &structs.ServiceCheck{
		Name:     "test-restarts",
		Type:     structs.ServiceCheckTCP,
		Interval: 50 * time.Millisecond,
		CheckRestart: &structs.CheckRestart{
			Limit: 2,
			Grace: 100 * time.Millisecond,
		},
	}

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()

	// Replace mock Consul ServiceClient, with the real ServiceClient
	// backed by a mock consul whose checks are always unhealthy.
	consulAgent := agentconsul.NewMockAgent()
	consulAgent.SetStatus("critical")
	consulClient := agentconsul.NewServiceClient(consulAgent, conf.Logger, true)
	go consulClient.Run()
	defer consulClient.Shutdown()

	conf.Consul = consulClient

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)

	expectedEvents := []string{
		"Received",
		"Task Setup",
		"Started",
		"Restart Signaled",
		"Terminated",
		"Restarting",
		"Started",
		"Restart Signaled",
		"Terminated",
		"Restarting",
		"Started",
		"Restart Signaled",
		"Terminated",
		"Not Restarting",
	}

	// Bump maxEvents so task events aren't dropped
	tr.maxEvents = 100

	go tr.Run()
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait until the task exits. Don't simply wait for it to run as it may
	// get restarted and terminated before the test is able to observe it
	// running.
	select {
	case <-tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		require.Fail(t, "timeout")
	}

	state := tr.TaskState()
	actualEvents := make([]string, len(state.Events))
	for i, e := range state.Events {
		actualEvents[i] = string(e.Type)
	}
	require.Equal(t, actualEvents, expectedEvents)

	require.Equal(t, structs.TaskStateDead, state.State)
	require.True(t, state.Failed, pretty.Sprint(state))
}

// testWaitForTaskToStart waits for the task to be running or fails the test
func testWaitForTaskToStart(t *testing.T, tr *TaskRunner) {
	testutil.WaitForResult(func() (bool, error) {
		ts := tr.TaskState()
		return ts.State == structs.TaskStateRunning, fmt.Errorf("%v", ts.State)
	}, func(err error) {
		require.NoError(t, err)
	})
}
