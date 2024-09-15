// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/snappy"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/getter"
	"github.com/hashicorp/nomad/client/config"
	consulclient "github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/devicemanager"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/lib/proclib"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	regMock "github.com/hashicorp/nomad/client/serviceregistration/mock"
	"github.com/hashicorp/nomad/client/serviceregistration/wrapper"
	cstate "github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/client/widmgr"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	mockdriver "github.com/hashicorp/nomad/drivers/mock"
	"github.com/hashicorp/nomad/drivers/rawexec"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
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
func testTaskRunnerConfig(t *testing.T, alloc *structs.Allocation, taskName string, vault vaultclient.VaultClient) (*Config, func()) {
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
	allocDir := allocdir.NewAllocDir(logger, clientConf.AllocDir, clientConf.AllocMountsDir, alloc.ID)
	if err := allocDir.Build(); err != nil {
		cleanup()
		t.Fatalf("error building alloc dir: %v", err)
	}
	taskDir := allocDir.NewTaskDir(thisTask)

	// Create cgroup
	f := cgroupslib.Factory(alloc.ID, taskName, false)
	must.NoError(t, f.Setup())

	trCleanup := func() {
		// destroy and remove the cgroup
		_ = f.Kill()
		_ = f.Teardown()
		// destroy the alloc dir
		if err := allocDir.Destroy(); err != nil {
			t.Logf("error destroying alloc dir: %v", err)
		}
		cleanup()
	}

	shutdownDelayCtx, shutdownDelayCancelFn := context.WithCancel(context.Background())

	// Create a closed channel to mock TaskCoordinator.startConditionForTask.
	// Closed channel indicates this task is not blocked on prestart hooks.
	closedCh := make(chan struct{})
	close(closedCh)

	// Set up the Nomad and Consul registration providers along with the wrapper.
	consulRegMock := regMock.NewServiceRegistrationHandler(logger)
	nomadRegMock := regMock.NewServiceRegistrationHandler(logger)
	wrapperMock := wrapper.NewHandlerWrapper(logger, consulRegMock, nomadRegMock)

	widsigner := widmgr.NewMockWIDSigner(thisTask.Identities)
	db := cstate.NewMemDB(logger)

	if thisTask.Vault != nil {
		clientConf.GetDefaultVault().Enabled = pointer.Of(true)
	}

	var vaultFunc vaultclient.VaultClientFunc
	if vault != nil {
		vaultFunc = func(_ string) (vaultclient.VaultClient, error) { return vault, nil }
	}
	// the envBuilder for the WIDMgr never has access to the task, so don't
	// include it here
	envBuilder := taskenv.NewBuilder(mock.Node(), alloc, nil, "global")

	conf := &Config{
		Alloc:                 alloc,
		ClientConfig:          clientConf,
		Task:                  thisTask,
		TaskDir:               taskDir,
		Logger:                clientConf.Logger,
		ConsulServices:        consulRegMock,
		ConsulSI:              consulclient.NewMockServiceIdentitiesClient(),
		VaultFunc:             vaultFunc,
		StateDB:               cstate.NoopDB{},
		StateUpdater:          NewMockTaskStateUpdater(),
		DeviceManager:         devicemanager.NoopMockManager(),
		DriverManager:         drivermanager.TestDriverManager(t),
		ServersContactedCh:    make(chan struct{}),
		StartConditionMetCh:   closedCh,
		ShutdownDelayCtx:      shutdownDelayCtx,
		ShutdownDelayCancelFn: shutdownDelayCancelFn,
		ServiceRegWrapper:     wrapperMock,
		Getter:                getter.TestSandbox(t),
		Wranglers:             proclib.MockWranglers(t),
		WIDMgr:                widmgr.NewWIDMgr(widsigner, alloc, db, logger, envBuilder),
		AllocHookResources:    cstructs.NewAllocHookResources(),
	}

	// The alloc runner identity_hook is responsible for running the WIDMgr, so
	// run it before returning to simulate that.
	if err := conf.WIDMgr.Run(); err != nil {
		trCleanup()
		t.Fatalf("failed to run WIDMgr: %v", err)
	}

	return conf, trCleanup
}

// runTestTaskRunner runs a TaskRunner and returns its configuration as well as
// a cleanup function that ensures the runner is stopped and cleaned up. Tests
// which need to change the Config *must* use testTaskRunnerConfig instead.
func runTestTaskRunner(t *testing.T, alloc *structs.Allocation, taskName string) (*TaskRunner, *Config, func()) {
	config, cleanup := testTaskRunnerConfig(t, alloc, taskName, nil)

	// This is usually handled by the identity hook in the alloc runner, so it
	// must be called manually when testing a task runner in isolation.
	if config.WIDMgr != nil {
		err := config.WIDMgr.Run()
		must.NoError(t, err)
	}

	tr, err := NewTaskRunner(config)

	require.NoError(t, err)
	go tr.Run()

	return tr, config, func() {
		tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
		cleanup()
	}
}

func TestTaskRunner_BuildTaskConfig_CPU_Memory(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name                  string
		cpu                   int64
		memoryMB              int64
		memoryMaxMB           int64
		expectedLinuxMemoryMB int64
	}{
		{
			name:                  "plain no max",
			cpu:                   100,
			memoryMB:              100,
			memoryMaxMB:           0,
			expectedLinuxMemoryMB: 100,
		},
		{
			name:                  "plain with max=reserve",
			cpu:                   100,
			memoryMB:              100,
			memoryMaxMB:           100,
			expectedLinuxMemoryMB: 100,
		},
		{
			name:                  "plain with max>reserve",
			cpu:                   100,
			memoryMB:              100,
			memoryMaxMB:           200,
			expectedLinuxMemoryMB: 200,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			alloc := mock.BatchAlloc()
			alloc.Job.TaskGroups[0].Count = 1
			task := alloc.Job.TaskGroups[0].Tasks[0]
			task.Driver = "mock_driver"
			task.Config = map[string]interface{}{
				"run_for": "2s",
			}
			res := alloc.AllocatedResources.Tasks[task.Name]
			res.Cpu.CpuShares = c.cpu
			res.Memory.MemoryMB = c.memoryMB
			res.Memory.MemoryMaxMB = c.memoryMaxMB

			conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
			conf.StateDB = cstate.NewMemDB(conf.Logger) // "persist" state between task runners
			defer cleanup()

			// Run the first TaskRunner
			tr, err := NewTaskRunner(conf)
			require.NoError(t, err)

			tc := tr.buildTaskConfig()
			require.Equal(t, c.cpu, tc.Resources.LinuxResources.CPUShares)
			require.Equal(t, c.expectedLinuxMemoryMB*1024*1024, tc.Resources.LinuxResources.MemoryLimitBytes)

			require.Equal(t, c.cpu, tc.Resources.NomadResources.Cpu.CpuShares)
			require.Equal(t, c.memoryMB, tc.Resources.NomadResources.Memory.MemoryMB)
			require.Equal(t, c.memoryMaxMB, tc.Resources.NomadResources.Memory.MemoryMaxMB)
		})
	}
}

// TestTaskRunner_Stop_ExitCode asserts that the exit code is captured on a task, even if it's stopped
func TestTaskRunner_Stop_ExitCode(t *testing.T) {
	ctestutil.ExecCompatible(t)
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	alloc.Job.TaskGroups[0].Count = 1
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.KillSignal = "SIGTERM"
	task.Driver = "raw_exec"
	task.Config = map[string]interface{}{
		"command": "/bin/sleep",
		"args":    []string{"1000"},
	}
	task.Env = map[string]string{
		"NOMAD_PARENT_CGROUP": "nomad.slice",
		"NOMAD_ALLOC_ID":      alloc.ID,
		"NOMAD_TASK_NAME":     task.Name,
	}

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	defer cleanup()

	// Run the first TaskRunner
	tr, err := NewTaskRunner(conf)

	require.NoError(t, err)
	go tr.Run()

	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait for it to be running
	testWaitForTaskToStart(t, tr)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = tr.Kill(ctx, structs.NewTaskEvent("shutdown"))
	require.NoError(t, err)

	var exitEvent *structs.TaskEvent
	state := tr.TaskState()
	for _, e := range state.Events {
		if e.Type == structs.TaskTerminated {
			exitEvent = e
			break
		}
	}
	require.NotNilf(t, exitEvent, "exit event not found: %v", state.Events)

	require.Equal(t, 143, exitEvent.ExitCode)
	require.Equal(t, 15, exitEvent.Signal)

}

// TestTaskRunner_Restore_Running asserts restoring a running task does not
// rerun the task.
func TestTaskRunner_Restore_Running(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	alloc := mock.BatchAlloc()
	alloc.Job.TaskGroups[0].Count = 1
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "2s",
	}
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	conf.StateDB = cstate.NewMemDB(conf.Logger) // "persist" state between task runners
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
	testWaitForTaskToDie(t, newTR)

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

// TestTaskRunner_Restore_Dead asserts that restoring a dead task will place it
// back in the correct state. If the task was waiting for an alloc restart it
// must be able to be restarted after restore, otherwise a restart must fail.
func TestTaskRunner_Restore_Dead(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	alloc.Job.TaskGroups[0].Count = 1
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "2s",
	}
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	conf.StateDB = cstate.NewMemDB(conf.Logger) // "persist" state between task runners
	defer cleanup()

	// Run the first TaskRunner
	origTR, err := NewTaskRunner(conf)
	require.NoError(t, err)
	go origTR.Run()
	defer origTR.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait for it to be dead
	testWaitForTaskToDie(t, origTR)

	// Cause TR to exit without shutting down task
	origTR.Shutdown()

	// Start a new TaskRunner and do the Restore
	newTR, err := NewTaskRunner(conf)
	require.NoError(t, err)
	require.NoError(t, newTR.Restore())

	go newTR.Run()
	defer newTR.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Verify that the TaskRunner is still active since it was recovered after
	// a forced shutdown.
	select {
	case <-newTR.WaitCh():
		require.Fail(t, "WaitCh is not blocking")
	default:
	}

	// Verify that we can restart task.
	// Retry a few times as the newTR.Run() may not have started yet.
	testutil.WaitForResult(func() (bool, error) {
		ev := &structs.TaskEvent{Type: structs.TaskRestartSignal}
		err = newTR.ForceRestart(context.Background(), ev, false)
		return err == nil, err
	}, func(err error) {
		require.NoError(t, err)
	})
	testWaitForTaskToStart(t, newTR)

	// Kill task to verify that it's restored as dead and not able to restart.
	newTR.Kill(context.Background(), nil)
	testutil.WaitForResult(func() (bool, error) {
		select {
		case <-newTR.WaitCh():
			return true, nil
		default:
			return false, fmt.Errorf("task still running")
		}
	}, func(err error) {
		require.NoError(t, err)
	})

	newTR2, err := NewTaskRunner(conf)
	require.NoError(t, err)
	require.NoError(t, newTR2.Restore())

	go newTR2.Run()
	defer newTR2.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	ev := &structs.TaskEvent{Type: structs.TaskRestartSignal}
	err = newTR2.ForceRestart(context.Background(), ev, false)
	require.Equal(t, err, ErrTaskNotRunning)
}

// setupRestoreFailureTest starts a service, shuts down the task runner, and
// kills the task before restarting a new TaskRunner. The new TaskRunner is
// returned once it is running and waiting in pending along with a cleanup
// func.
func setupRestoreFailureTest(t *testing.T, alloc *structs.Allocation) (*TaskRunner, *Config, func()) {
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "raw_exec"
	task.Config = map[string]interface{}{
		"command": "sleep",
		"args":    []string{"30"},
	}
	task.Env = map[string]string{
		"NOMAD_PARENT_CGROUP": "nomad.slice",
		"NOMAD_ALLOC_ID":      alloc.ID,
		"NOMAD_TASK_NAME":     task.Name,
	}
	conf, cleanup1 := testTaskRunnerConfig(t, alloc, task.Name, nil)
	conf.StateDB = cstate.NewMemDB(conf.Logger) // "persist" state between runs

	// Run the first TaskRunner
	origTR, err := NewTaskRunner(conf)
	require.NoError(t, err)
	go origTR.Run()
	cleanup2 := func() {
		origTR.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
		cleanup1()
	}

	// Wait for it to be running
	testWaitForTaskToStart(t, origTR)

	handle := origTR.getDriverHandle()
	require.NotNil(t, handle)
	taskID := handle.taskID

	// Cause TR to exit without shutting down task
	origTR.Shutdown()

	// Get the driver
	driverPlugin, err := conf.DriverManager.Dispense(rawexec.PluginID.Name)
	require.NoError(t, err)
	rawexecDriver := driverPlugin.(*rawexec.Driver)

	// Assert the task is still running despite TR having exited
	taskStatus, err := rawexecDriver.InspectTask(taskID)
	require.NoError(t, err)
	require.Equal(t, drivers.TaskStateRunning, taskStatus.State)

	// Kill the task so it fails to recover when restore is called
	require.NoError(t, rawexecDriver.DestroyTask(taskID, true))
	_, err = rawexecDriver.InspectTask(taskID)
	require.EqualError(t, err, drivers.ErrTaskNotFound.Error())

	// Create a new TaskRunner and Restore the task
	conf.ServersContactedCh = make(chan struct{})
	newTR, err := NewTaskRunner(conf)
	require.NoError(t, err)

	// Assert the TR will wait on servers because reattachment failed
	require.NoError(t, newTR.Restore())
	require.True(t, newTR.waitOnServers)

	// Start new TR
	go newTR.Run()
	cleanup3 := func() {
		newTR.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
		cleanup2()
		cleanup1()
	}

	// Assert task has not been restarted
	_, err = rawexecDriver.InspectTask(taskID)
	require.EqualError(t, err, drivers.ErrTaskNotFound.Error())
	ts := newTR.TaskState()
	require.Equal(t, structs.TaskStatePending, ts.State)

	return newTR, conf, cleanup3
}

// TestTaskRunner_Restore_Restart asserts restoring a dead task blocks until
// MarkAlive is called. #1795
func TestTaskRunner_Restore_Restart(t *testing.T) {
	ci.Parallel(t)

	newTR, conf, cleanup := setupRestoreFailureTest(t, mock.Alloc())
	defer cleanup()

	// Fake contacting the server by closing the chan
	close(conf.ServersContactedCh)

	testutil.WaitForResult(func() (bool, error) {
		ts := newTR.TaskState().State
		return ts == structs.TaskStateRunning, fmt.Errorf("expected task to be running but found %q", ts)
	}, func(err error) {
		require.NoError(t, err)
	})
}

// TestTaskRunner_Restore_Kill asserts restoring a dead task blocks until
// the task is killed. #1795
func TestTaskRunner_Restore_Kill(t *testing.T) {
	ci.Parallel(t)

	newTR, _, cleanup := setupRestoreFailureTest(t, mock.Alloc())
	defer cleanup()

	// Sending the task a terminal update shouldn't kill it or unblock it
	alloc := newTR.Alloc().Copy()
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	newTR.Update(alloc)

	require.Equal(t, structs.TaskStatePending, newTR.TaskState().State)

	// AllocRunner will immediately kill tasks after sending a terminal
	// update.
	newTR.Kill(context.Background(), structs.NewTaskEvent(structs.TaskKilling))

	select {
	case <-newTR.WaitCh():
		// It died as expected!
	case <-time.After(10 * time.Second):
		require.Fail(t, "timeout waiting for task to die")
	}
}

// TestTaskRunner_Restore_Update asserts restoring a dead task blocks until
// Update is called. #1795
func TestTaskRunner_Restore_Update(t *testing.T) {
	ci.Parallel(t)

	newTR, conf, cleanup := setupRestoreFailureTest(t, mock.Alloc())
	defer cleanup()

	// Fake Client.runAllocs behavior by calling Update then closing chan
	alloc := newTR.Alloc().Copy()
	newTR.Update(alloc)

	// Update alone should not unblock the test
	require.Equal(t, structs.TaskStatePending, newTR.TaskState().State)

	// Fake Client.runAllocs behavior of closing chan after Update
	close(conf.ServersContactedCh)

	testutil.WaitForResult(func() (bool, error) {
		ts := newTR.TaskState().State
		return ts == structs.TaskStateRunning, fmt.Errorf("expected task to be running but found %q", ts)
	}, func(err error) {
		require.NoError(t, err)
	})
}

// TestTaskRunner_Restore_System asserts restoring a dead system task does not
// block.
func TestTaskRunner_Restore_System(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	alloc.Job.Type = structs.JobTypeSystem
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "raw_exec"
	task.Config = map[string]interface{}{
		"command": "sleep",
		"args":    []string{"30"},
	}
	task.Env = map[string]string{
		"NOMAD_PARENT_CGROUP": "nomad.slice",
		"NOMAD_ALLOC_ID":      alloc.ID,
		"NOMAD_TASK_NAME":     task.Name,
	}
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	defer cleanup()
	conf.StateDB = cstate.NewMemDB(conf.Logger) // "persist" state between runs

	// Run the first TaskRunner
	origTR, err := NewTaskRunner(conf)
	require.NoError(t, err)
	go origTR.Run()
	defer origTR.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait for it to be running
	testWaitForTaskToStart(t, origTR)

	handle := origTR.getDriverHandle()
	require.NotNil(t, handle)
	taskID := handle.taskID

	// Cause TR to exit without shutting down task
	origTR.Shutdown()

	// Get the driver
	driverPlugin, err := conf.DriverManager.Dispense(rawexec.PluginID.Name)
	require.NoError(t, err)
	rawexecDriver := driverPlugin.(*rawexec.Driver)

	// Assert the task is still running despite TR having exited
	taskStatus, err := rawexecDriver.InspectTask(taskID)
	require.NoError(t, err)
	require.Equal(t, drivers.TaskStateRunning, taskStatus.State)

	// Kill the task so it fails to recover when restore is called
	require.NoError(t, rawexecDriver.DestroyTask(taskID, true))
	_, err = rawexecDriver.InspectTask(taskID)
	require.EqualError(t, err, drivers.ErrTaskNotFound.Error())

	// Create a new TaskRunner and Restore the task
	conf.ServersContactedCh = make(chan struct{})
	newTR, err := NewTaskRunner(conf)
	require.NoError(t, err)

	// Assert the TR will not wait on servers even though reattachment
	// failed because it is a system task.
	require.NoError(t, newTR.Restore())
	require.False(t, newTR.waitOnServers)

	// Nothing should have closed the chan
	select {
	case <-conf.ServersContactedCh:
		require.Fail(t, "serversContactedCh was closed but should not have been")
	default:
	}

	testutil.WaitForResult(func() (bool, error) {
		ts := newTR.TaskState().State
		return ts == structs.TaskStateRunning, fmt.Errorf("expected task to be running but found %q", ts)
	}, func(err error) {
		require.NoError(t, err)
	})
}

// TestTaskRunner_MarkFailedKill asserts that MarkFailedKill marks the task as failed
// and cancels the killCtx so a subsequent Run() will do any necessary task cleanup.
func TestTaskRunner_MarkFailedKill(t *testing.T) {
	ci.Parallel(t)

	// set up some taskrunner
	alloc := mock.MinAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	t.Cleanup(cleanup)
	tr, err := NewTaskRunner(conf)
	must.NoError(t, err)

	// side quest: set this lifecycle coordination channel,
	// so early in tr MAIN, it doesn't randomly follow that route.
	// test config creates this already closed, but not so in real life.
	startCh := make(chan struct{})
	t.Cleanup(func() { close(startCh) })
	tr.startConditionMetCh = startCh

	// function under test: should mark the task as failed and cancel kill context
	reason := "because i said so"
	tr.MarkFailedKill(reason)

	// explicitly check kill context.
	select {
	case <-tr.killCtx.Done():
	default:
		t.Fatal("kill context should be done")
	}

	// Run() should now follow the kill path.
	go tr.Run()

	select { // it should finish up very quickly
	case <-tr.WaitCh():
	case <-time.After(time.Second):
		t.Error("task not killed (or not as fast as expected)")
	}

	// check state for expected values and events
	state := tr.TaskState()

	// this gets set directly by MarkFailedKill()
	test.True(t, state.Failed, test.Sprint("task should have failed"))
	// this is set in Run()
	test.Eq(t, structs.TaskStateDead, state.State, test.Sprint("task should be dead"))
	// reason "because i said so" should be a task event message
	foundMessages := make(map[string]bool)
	for _, event := range state.Events {
		foundMessages[event.DisplayMessage] = true
	}
	test.True(t, foundMessages[reason], test.Sprintf("expected '%s' in events: %#v", reason, foundMessages))
}

// TestTaskRunner_TaskEnv_Interpolated asserts driver configurations are
// interpolated.
func TestTaskRunner_TaskEnv_Interpolated(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	alloc := mock.BatchAlloc()
	alloc.Job.TaskGroups[0].Meta = map[string]string{
		"common_user": "somebody",
	}
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Meta = map[string]string{
		"foo": "bar",
	}

	// Use interpolation from both node attributes and meta vars
	task.Config = map[string]interface{}{
		"run_for":       "1ms",
		"stdout_string": `${node.region} ${NOMAD_META_foo} ${NOMAD_META_common_user}`,
	}

	tr, conf, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

	// Wait for task to complete
	testWaitForTaskToDie(t, tr)

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

// TestTaskRunner_TaskEnv_None asserts raw_exec uses host paths and env vars.
func TestTaskRunner_TaskEnv_None(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "raw_exec"
	task.Config = map[string]interface{}{
		"command": "sh",
		"args": []string{"-c", "echo $NOMAD_ALLOC_DIR; " +
			"echo $NOMAD_TASK_DIR; " +
			"echo $NOMAD_SECRETS_DIR; " +
			"echo $PATH",
		},
	}
	task.Env = map[string]string{
		"NOMAD_PARENT_CGROUP": "nomad.slice",
		"NOMAD_ALLOC_ID":      alloc.ID,
		"NOMAD_TASK_NAME":     task.Name,
	}
	tr, conf, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

	// Expect host paths
	root := filepath.Join(conf.ClientConfig.AllocDir, alloc.ID)
	taskDir := filepath.Join(root, task.Name)
	exp := fmt.Sprintf(`%s/alloc
%s/local
%s/secrets
%s
`, root, taskDir, taskDir, os.Getenv("PATH"))

	// Wait for task to exit and kill the task runner to run the stop hooks.
	testWaitForTaskToDie(t, tr)
	tr.Kill(context.Background(), structs.NewTaskEvent("kill"))
	select {
	case <-tr.WaitCh():
	case <-time.After(15 * time.Second):
		require.Fail("timeout waiting for task to exit")
	}

	// Read stdout
	p := filepath.Join(conf.TaskDir.LogDir, task.Name+".stdout.0")
	stdout, err := os.ReadFile(p)
	require.NoError(err)
	require.Equalf(exp, string(stdout), "expected: %s\n\nactual: %s\n", exp, stdout)
}

// Test that devices get sent to the driver
func TestTaskRunner_DevicePropogation(t *testing.T) {
	ci.Parallel(t)
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

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	conf.StateDB = cstate.NewMemDB(conf.Logger) // "persist" state between task runners
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
	testWaitForTaskToDie(t, tr)

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
	ci.Parallel(t)
	require := require.New(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	conf.StateDB = cstate.NewMemDB(conf.Logger) // "persist" state between prestart calls
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
	ci.Parallel(t)
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

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	conf.StateDB = cstate.NewMemDB(conf.Logger) // "persist" state between prestart calls
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
	ci.Parallel(t)

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Services[0].Tags = []string{"tag1"}
	task.Services = task.Services[:1] // only need 1 for this test
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1000s",
	}

	// No shutdown escape hatch for this delay, so don't set it too high
	task.ShutdownDelay = 1000 * time.Duration(testutil.TestMultiplier()) * time.Millisecond

	tr, conf, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

	mockConsul := conf.ConsulServices.(*regMock.ServiceRegistrationHandler)

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

	// Wait for *1* de-registration calls (all [non-]canary variants removed).

WAIT:
	for {
		ops := mockConsul.GetOps()
		switch n := len(ops); n {
		case 1:
			// Waiting for single de-registration call.
		case 2:
			require.Equalf(t, "remove", ops[1].Op, "expected deregistration but found: %#v", ops[1])
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

// TestTaskRunner_NoShutdownDelay asserts services are removed from
// Consul and tasks are killed without waiting for ${shutdown_delay}
// when the alloc has the NoShutdownDelay transition flag set.
func TestTaskRunner_NoShutdownDelay(t *testing.T) {
	ci.Parallel(t)

	// don't set this too high so that we don't block the test runner
	// on shutting down the agent if the test fails
	maxTestDuration := time.Duration(testutil.TestMultiplier()*10) * time.Second
	maxTimeToFailDuration := time.Duration(testutil.TestMultiplier()) * time.Second

	alloc := mock.Alloc()
	alloc.DesiredTransition = structs.DesiredTransition{NoShutdownDelay: pointer.Of(true)}
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Services[0].Tags = []string{"tag1"}
	task.Services = task.Services[:1] // only need 1 for this test
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1000s",
	}
	task.ShutdownDelay = maxTestDuration

	tr, conf, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

	mockConsul := conf.ConsulServices.(*regMock.ServiceRegistrationHandler)

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

	testCtx, cancel := context.WithTimeout(context.Background(), maxTimeToFailDuration)
	defer cancel()

	killed := make(chan error)
	go func() {
		tr.shutdownDelayCancel()
		err := tr.Kill(testCtx, structs.NewTaskEvent("test"))
		killed <- err
	}()

	// Wait for first de-registration call. Note that unlike
	// TestTaskRunner_ShutdownDelay, we're racing with task exit
	// and can't assert that we only get the first deregistration op
	// (from serviceHook.PreKill).
	testutil.WaitForResult(func() (bool, error) {
		ops := mockConsul.GetOps()
		if n := len(ops); n < 2 {
			return false, fmt.Errorf("expected at least 2 consul operations.")
		}
		return ops[1].Op == "remove", fmt.Errorf(
			"consul operation was not a deregistration: %#v", ops[1])
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Wait for the task to exit
	select {
	case <-tr.WaitCh():
	case <-time.After(maxTimeToFailDuration):
		t.Fatalf("task kill did not ignore shutdown delay")
		return
	}

	err := <-killed
	must.NoError(t, err)

	// Check that we only emit the expected events.
	hasEvent := false
	for _, ev := range tr.state.Events {
		must.NotEq(t, structs.TaskWaitingShuttingDownDelay, ev.Type)
		if ev.Type == structs.TaskSkippingShutdownDelay {
			hasEvent = true
		}
	}
	must.True(t, hasEvent)
}

// TestTaskRunner_Dispatch_Payload asserts that a dispatch job runs and the
// payload was written to disk.
func TestTaskRunner_Dispatch_Payload(t *testing.T) {
	ci.Parallel(t)

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

	tr, _, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

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
	data, err := os.ReadFile(payloadPath)
	require.NoError(t, err)
	require.Equal(t, expected, data)
}

// TestTaskRunner_SignalFailure asserts that signal errors are properly
// propagated from the driver to TaskRunner.
func TestTaskRunner_SignalFailure(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	errMsg := "test forcing failure"
	task.Config = map[string]interface{}{
		"run_for":      "10m",
		"signal_error": errMsg,
	}

	tr, _, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

	testWaitForTaskToStart(t, tr)

	require.EqualError(t, tr.Signal(&structs.TaskEvent{}, "SIGINT"), errMsg)
}

// TestTaskRunner_RestartTask asserts that restarting a task works and emits a
// Restarting event.
func TestTaskRunner_RestartTask(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10m",
	}

	tr, _, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

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
	ci.Parallel(t)

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
	task.Services[0].Provider = structs.ServiceProviderConsul

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	defer cleanup()

	// Replace mock Consul ServiceClient, with the real ServiceClient
	// backed by a mock consul whose checks are always unhealthy.
	consulAgent := agentconsul.NewMockAgent(agentconsul.Features{
		Enterprise: false,
		Namespaces: false,
	})
	consulAgent.SetStatus("critical")
	namespacesClient := agentconsul.NewNamespacesClient(agentconsul.NewMockNamespaces(nil), consulAgent)
	consulServices := agentconsul.NewServiceClient(consulAgent, namespacesClient, conf.Logger, true)
	go consulServices.Run()
	defer consulServices.Shutdown()

	conf.ConsulServices = consulServices
	conf.ServiceRegWrapper = wrapper.NewHandlerWrapper(conf.Logger, consulServices, nil)

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
	testWaitForTaskToDie(t, tr)

	state := tr.TaskState()
	actualEvents := make([]string, len(state.Events))
	for i, e := range state.Events {
		actualEvents[i] = string(e.Type)
	}
	require.Equal(t, actualEvents, expectedEvents)
	require.Equal(t, structs.TaskStateDead, state.State)
	require.True(t, state.Failed, pretty.Sprint(state))
}

type mockEnvoyBootstrapHook struct {
	// nothing
}

func (_ *mockEnvoyBootstrapHook) Name() string {
	return "mock_envoy_bootstrap"
}

func (_ *mockEnvoyBootstrapHook) Prestart(_ context.Context, _ *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	resp.Done = true
	return nil
}

// The envoy bootstrap hook tries to connect to consul and run the envoy
// bootstrap command, so turn it off when testing connect jobs that are not
// using envoy.
func useMockEnvoyBootstrapHook(tr *TaskRunner) {
	mock := new(mockEnvoyBootstrapHook)
	for i, hook := range tr.runnerHooks {
		if _, ok := hook.(*envoyBootstrapHook); ok {
			tr.runnerHooks[i] = mock
		}
	}
}

// TestTaskRunner_BlockForSIDSToken asserts tasks do not start until a Consul
// Service Identity token is derived.
func TestTaskRunner_BlockForSIDSToken(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	alloc := mock.BatchConnectAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Config = map[string]interface{}{
		"run_for": "0s",
	}

	trConfig, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	defer cleanup()

	// set a consul token on the Nomad client's consul config, because that is
	// what gates the action of requesting SI token(s)
	trConfig.ClientConfig.GetDefaultConsul().Token = uuid.Generate()

	// control when we get a Consul SI token
	token := uuid.Generate()
	waitCh := make(chan struct{})
	deriveFn := func(context.Context, *structs.Allocation, []string) (map[string]string, error) {
		<-waitCh
		return map[string]string{task.Name: token}, nil
	}
	siClient := trConfig.ConsulSI.(*consulclient.MockServiceIdentitiesClient)
	siClient.DeriveTokenFn = deriveFn

	// start the task runner
	tr, err := NewTaskRunner(trConfig)
	r.NoError(err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	useMockEnvoyBootstrapHook(tr) // mock the envoy bootstrap hook

	go tr.Run()

	// assert task runner blocks on SI token
	select {
	case <-tr.WaitCh():
		r.Fail("task_runner exited before si unblocked")
	case <-time.After(100 * time.Millisecond):
	}

	// assert task state is still pending
	r.Equal(structs.TaskStatePending, tr.TaskState().State)

	// unblock service identity token
	close(waitCh)

	// task runner should exit now that it has been unblocked and it is a batch
	// job with a zero sleep time
	testWaitForTaskToDie(t, tr)

	// assert task exited successfully
	finalState := tr.TaskState()
	r.Equal(structs.TaskStateDead, finalState.State)
	r.False(finalState.Failed)

	// assert the token is on disk
	tokenPath := filepath.Join(trConfig.TaskDir.SecretsDir, sidsTokenFile)
	data, err := os.ReadFile(tokenPath)
	r.NoError(err)
	r.Equal(token, string(data))
}

func TestTaskRunner_DeriveSIToken_Retry(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	alloc := mock.BatchConnectAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Config = map[string]interface{}{
		"run_for": "0s",
	}

	trConfig, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	defer cleanup()

	// set a consul token on the Nomad client's consul config, because that is
	// what gates the action of requesting SI token(s)
	trConfig.ClientConfig.GetDefaultConsul().Token = uuid.Generate()

	// control when we get a Consul SI token (recoverable failure on first call)
	token := uuid.Generate()
	deriveCount := 0
	deriveFn := func(context.Context, *structs.Allocation, []string) (map[string]string, error) {
		if deriveCount > 0 {

			return map[string]string{task.Name: token}, nil
		}
		deriveCount++
		return nil, structs.NewRecoverableError(errors.New("try again later"), true)
	}
	siClient := trConfig.ConsulSI.(*consulclient.MockServiceIdentitiesClient)
	siClient.DeriveTokenFn = deriveFn

	// start the task runner
	tr, err := NewTaskRunner(trConfig)
	r.NoError(err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	useMockEnvoyBootstrapHook(tr) // mock the envoy bootstrap
	go tr.Run()

	// assert task runner blocks on SI token
	testWaitForTaskToDie(t, tr)

	// assert task exited successfully
	finalState := tr.TaskState()
	r.Equal(structs.TaskStateDead, finalState.State)
	r.False(finalState.Failed)

	// assert the token is on disk
	tokenPath := filepath.Join(trConfig.TaskDir.SecretsDir, sidsTokenFile)
	data, err := os.ReadFile(tokenPath)
	r.NoError(err)
	r.Equal(token, string(data))
}

// TestTaskRunner_DeriveSIToken_Unrecoverable asserts that an unrecoverable error
// from deriving a service identity token will fail a task.
func TestTaskRunner_DeriveSIToken_Unrecoverable(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	alloc := mock.BatchConnectAlloc()
	tg := alloc.Job.TaskGroups[0]
	tg.RestartPolicy.Attempts = 0
	tg.RestartPolicy.Interval = 0
	tg.RestartPolicy.Delay = 0
	tg.RestartPolicy.Mode = structs.RestartPolicyModeFail
	task := tg.Tasks[0]
	task.Config = map[string]interface{}{
		"run_for": "0s",
	}

	trConfig, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	defer cleanup()

	// set a consul token on the Nomad client's consul config, because that is
	// what gates the action of requesting SI token(s)
	trConfig.ClientConfig.GetDefaultConsul().Token = uuid.Generate()

	// SI token derivation suffers a non-retryable error
	siClient := trConfig.ConsulSI.(*consulclient.MockServiceIdentitiesClient)
	siClient.SetDeriveTokenError(alloc.ID, []string{task.Name}, errors.New("non-recoverable"))

	tr, err := NewTaskRunner(trConfig)
	r.NoError(err)

	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	useMockEnvoyBootstrapHook(tr) // mock the envoy bootstrap hook
	go tr.Run()

	// Wait for the task to die
	select {
	case <-tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		require.Fail(t, "timed out waiting for task runner to fail")
	}

	// assert we have died and failed
	finalState := tr.TaskState()
	r.Equal(structs.TaskStateDead, finalState.State)
	r.True(finalState.Failed)
	r.Equal(5, len(finalState.Events))
	/*
	 + event: Task received by client
	 + event: Building Task Directory
	 + event: consul: failed to derive SI token: non-recoverable
	 + event: consul_sids: context canceled
	 + event: Policy allows no restarts
	*/
	r.Equal("true", finalState.Events[2].Details["fails_task"])
}

// TestTaskRunner_BlockForVaultToken asserts tasks do not start until a vault token
// is derived.
func TestTaskRunner_BlockForVaultToken(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Config = map[string]interface{}{
		"run_for": "0s",
	}
	task.Vault = &structs.Vault{
		Cluster:  structs.VaultDefaultCluster,
		Policies: []string{"default"},
	}

	// Control when we get a Vault token
	token := "1234"
	waitCh := make(chan struct{})
	handler := func(*structs.Allocation, []string) (map[string]string, error) {
		<-waitCh
		return map[string]string{task.Name: token}, nil
	}

	vc, err := vaultclient.NewMockVaultClient(structs.VaultDefaultCluster)
	must.NoError(t, err)
	vaultClient := vc.(*vaultclient.MockVaultClient)
	vaultClient.DeriveTokenFn = handler

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, vaultClient)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	go tr.Run()

	// Assert TR blocks on vault token (does *not* exit)
	select {
	case <-tr.WaitCh():
		require.Fail(t, "tr exited before vault unblocked")
	case <-time.After(1 * time.Second):
	}

	// Assert task state is still Pending
	require.Equal(t, structs.TaskStatePending, tr.TaskState().State)

	// Unblock vault token
	close(waitCh)

	// TR should exit now that it's unblocked by vault as its a batch job
	// with 0 sleeping.
	testWaitForTaskToDie(t, tr)

	// Assert task exited successfully
	finalState := tr.TaskState()
	require.Equal(t, structs.TaskStateDead, finalState.State)
	require.False(t, finalState.Failed)

	// Check that the token is on disk
	tokenPath := filepath.Join(conf.TaskDir.PrivateDir, vaultTokenFile)
	data, err := os.ReadFile(tokenPath)
	require.NoError(t, err)
	require.Equal(t, token, string(data))

	tokenPath = filepath.Join(conf.TaskDir.SecretsDir, vaultTokenFile)
	data, err = os.ReadFile(tokenPath)
	require.NoError(t, err)
	require.Equal(t, token, string(data))

	// Kill task runner to trigger stop hooks
	tr.Kill(context.Background(), structs.NewTaskEvent("kill"))
	select {
	case <-tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		require.Fail(t, "timed out waiting for task runner to exit")
	}

	// Check the token was revoked
	testutil.WaitForResult(func() (bool, error) {
		if len(vaultClient.StoppedTokens()) != 1 {
			return false, fmt.Errorf("Expected a stopped token %q but found: %v", token, vaultClient.StoppedTokens())
		}

		if a := vaultClient.StoppedTokens()[0]; a != token {
			return false, fmt.Errorf("got stopped token %q; want %q", a, token)
		}
		return true, nil
	}, func(err error) {
		require.Fail(t, err.Error())
	})
}

func TestTaskRunner_DisableFileForVaultToken(t *testing.T) {
	ci.Parallel(t)

	// Create test allocation with a Vault block disabling the token file in
	// the secrets dir.
	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Config = map[string]any{
		"run_for": "0s",
	}
	task.Vault = &structs.Vault{
		Cluster:     structs.VaultDefaultCluster,
		Policies:    []string{"default"},
		DisableFile: true,
	}

	// Setup a test Vault client
	token := "1234"
	handler := func(*structs.Allocation, []string) (map[string]string, error) {
		return map[string]string{task.Name: token}, nil
	}
	vc, err := vaultclient.NewMockVaultClient(structs.VaultDefaultCluster)
	must.NoError(t, err)
	vaultClient := vc.(*vaultclient.MockVaultClient)
	vaultClient.DeriveTokenFn = handler

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, vaultClient)
	defer cleanup()

	// Start task runner and wait for it to complete.
	tr, err := NewTaskRunner(conf)
	must.NoError(t, err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	go tr.Run()

	testWaitForTaskToDie(t, tr)

	// Verify task exited successfully.
	finalState := tr.TaskState()
	must.Eq(t, structs.TaskStateDead, finalState.State)
	must.False(t, finalState.Failed)

	// Verify token is in the private dir.
	tokenPath := filepath.Join(conf.TaskDir.PrivateDir, vaultTokenFile)
	data, err := os.ReadFile(tokenPath)
	must.NoError(t, err)
	must.Eq(t, token, string(data))

	// Verify token is not in secrets dir.
	tokenPath = filepath.Join(conf.TaskDir.SecretsDir, vaultTokenFile)
	_, err = os.Stat(tokenPath)
	must.ErrorIs(t, err, os.ErrNotExist)
}

// TestTaskRunner_DeriveToken_Retry asserts that if a recoverable error is
// returned when deriving a vault token a task will continue to block while
// it's retried.
func TestTaskRunner_DeriveToken_Retry(t *testing.T) {
	ci.Parallel(t)
	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Vault = &structs.Vault{
		Cluster:  structs.VaultDefaultCluster,
		Policies: []string{"default"},
	}

	// Fail on the first attempt to derive a vault token
	token := "1234"
	count := 0
	handler := func(*structs.Allocation, []string) (map[string]string, error) {
		if count > 0 {
			return map[string]string{task.Name: token}, nil
		}

		count++
		return nil, structs.NewRecoverableError(fmt.Errorf("Want a retry"), true)
	}
	vc, err := vaultclient.NewMockVaultClient(structs.VaultDefaultCluster)
	must.NoError(t, err)
	vaultClient := vc.(*vaultclient.MockVaultClient)
	vaultClient.DeriveTokenFn = handler

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, vaultClient)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	go tr.Run()

	// Wait for TR to die and check its state
	testWaitForTaskToDie(t, tr)

	state := tr.TaskState()
	require.Equal(t, structs.TaskStateDead, state.State)
	require.False(t, state.Failed)

	// Kill task runner to trigger stop hooks
	tr.Kill(context.Background(), structs.NewTaskEvent("kill"))
	select {
	case <-tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		require.Fail(t, "timed out waiting for task runner to exit")
	}

	require.Equal(t, 1, count)

	// Check that the token is on disk
	tokenPath := filepath.Join(conf.TaskDir.PrivateDir, vaultTokenFile)
	data, err := os.ReadFile(tokenPath)
	require.NoError(t, err)
	require.Equal(t, token, string(data))

	// Check the token was revoked
	testutil.WaitForResult(func() (bool, error) {
		if len(vaultClient.StoppedTokens()) != 1 {
			return false, fmt.Errorf("Expected a stopped token: %v", vaultClient.StoppedTokens())
		}

		if a := vaultClient.StoppedTokens()[0]; a != token {
			return false, fmt.Errorf("got stopped token %q; want %q", a, token)
		}
		return true, nil
	}, func(err error) {
		require.Fail(t, err.Error())
	})
}

// TestTaskRunner_DeriveToken_Unrecoverable asserts that an unrecoverable error
// from deriving a vault token will fail a task.
func TestTaskRunner_DeriveToken_Unrecoverable(t *testing.T) {
	ci.Parallel(t)

	// Use a batch job with no restarts
	alloc := mock.BatchAlloc()
	tg := alloc.Job.TaskGroups[0]
	tg.RestartPolicy.Attempts = 0
	tg.RestartPolicy.Interval = 0
	tg.RestartPolicy.Delay = 0
	tg.RestartPolicy.Mode = structs.RestartPolicyModeFail
	task := tg.Tasks[0]
	task.Config = map[string]interface{}{
		"run_for": "0s",
	}
	task.Vault = &structs.Vault{
		Cluster:  structs.VaultDefaultCluster,
		Policies: []string{"default"},
	}

	// Error the token derivation
	vc, err := vaultclient.NewMockVaultClient(structs.VaultDefaultCluster)
	must.NoError(t, err)
	vaultClient := vc.(*vaultclient.MockVaultClient)
	vaultClient.SetDeriveTokenError(
		alloc.ID, []string{task.Name}, fmt.Errorf("Non recoverable"))

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, vaultClient)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	go tr.Run()

	// Wait for the task to die
	select {
	case <-tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		require.Fail(t, "timed out waiting for task runner to fail")
	}

	// Task should be dead and last event should have failed task
	state := tr.TaskState()
	require.Equal(t, structs.TaskStateDead, state.State)
	require.True(t, state.Failed)
	require.Len(t, state.Events, 3)
	require.True(t, state.Events[2].FailsTask)
}

// TestTaskRunner_Download_RawExec asserts that downloaded artifacts may be
// executed in a driver without filesystem isolation.
func TestTaskRunner_Download_RawExec(t *testing.T) {
	ci.SkipTestWithoutRootAccess(t)
	ci.Parallel(t)

	ts := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir("."))))
	defer ts.Close()

	// Create a task that downloads a script and executes it.
	alloc := mock.BatchAlloc()
	alloc.Job.TaskGroups[0].RestartPolicy = &structs.RestartPolicy{}

	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.RestartPolicy = &structs.RestartPolicy{}
	task.Driver = "raw_exec"
	task.Config = map[string]interface{}{
		"command": "noop.sh",
	}
	task.Env = map[string]string{
		"NOMAD_PARENT_CGROUP": "nomad.slice",
		"NOMAD_ALLOC_ID":      alloc.ID,
		"NOMAD_TASK_NAME":     task.Name,
	}
	task.Artifacts = []*structs.TaskArtifact{
		{
			GetterSource: fmt.Sprintf("%s/testdata/noop.sh", ts.URL),
			GetterMode:   "file",
			RelativeDest: "noop.sh",
		},
	}

	tr, _, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

	// Wait for task to run and exit
	testWaitForTaskToDie(t, tr)

	state := tr.TaskState()
	require.Equal(t, structs.TaskStateDead, state.State)
	require.False(t, state.Failed)
}

// TestTaskRunner_Download_List asserts that multiple artificats are downloaded
// before a task is run.
func TestTaskRunner_Download_List(t *testing.T) {
	ci.Parallel(t)
	ts := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir("."))))
	defer ts.Close()

	// Create an allocation that has a task with a list of artifacts.
	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	f1 := "task_runner_test.go"
	f2 := "task_runner.go"
	artifact1 := structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, f1),
	}
	artifact2 := structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, f2),
	}
	task.Artifacts = []*structs.TaskArtifact{&artifact1, &artifact2}

	tr, conf, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

	// Wait for task to run and exit
	testWaitForTaskToDie(t, tr)

	state := tr.TaskState()
	require.Equal(t, structs.TaskStateDead, state.State)
	require.False(t, state.Failed)

	require.Len(t, state.Events, 5)
	assert.Equal(t, structs.TaskReceived, state.Events[0].Type)
	assert.Equal(t, structs.TaskSetup, state.Events[1].Type)
	assert.Equal(t, structs.TaskDownloadingArtifacts, state.Events[2].Type)
	assert.Equal(t, structs.TaskStarted, state.Events[3].Type)
	assert.Equal(t, structs.TaskTerminated, state.Events[4].Type)

	// Check that both files exist.
	_, err := os.Stat(filepath.Join(conf.TaskDir.Dir, f1))
	require.NoErrorf(t, err, "%v not downloaded", f1)

	_, err = os.Stat(filepath.Join(conf.TaskDir.Dir, f2))
	require.NoErrorf(t, err, "%v not downloaded", f2)
}

// TestTaskRunner_Download_Retries asserts that failed artifact downloads are
// retried according to the task's restart policy.
func TestTaskRunner_Download_Retries(t *testing.T) {
	ci.Parallel(t)

	// Create an allocation that has a task with bad artifacts.
	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	artifact := structs.TaskArtifact{
		GetterSource: "http://127.0.0.1:0/foo/bar/baz",
	}
	task.Artifacts = []*structs.TaskArtifact{&artifact}

	// Make the restart policy retry once
	rp := &structs.RestartPolicy{
		Attempts: 1,
		Interval: 10 * time.Minute,
		Delay:    1 * time.Second,
		Mode:     structs.RestartPolicyModeFail,
	}
	alloc.Job.TaskGroups[0].RestartPolicy = rp
	alloc.Job.TaskGroups[0].Tasks[0].RestartPolicy = rp

	tr, _, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

	testWaitForTaskToDie(t, tr)

	state := tr.TaskState()
	require.Equal(t, structs.TaskStateDead, state.State)
	require.True(t, state.Failed)
	require.Len(t, state.Events, 8, pretty.Sprint(state.Events))
	require.Equal(t, structs.TaskReceived, state.Events[0].Type)
	require.Equal(t, structs.TaskSetup, state.Events[1].Type)
	require.Equal(t, structs.TaskDownloadingArtifacts, state.Events[2].Type)
	require.Equal(t, structs.TaskArtifactDownloadFailed, state.Events[3].Type)
	require.Equal(t, structs.TaskRestarting, state.Events[4].Type)
	require.Equal(t, structs.TaskDownloadingArtifacts, state.Events[5].Type)
	require.Equal(t, structs.TaskArtifactDownloadFailed, state.Events[6].Type)
	require.Equal(t, structs.TaskNotRestarting, state.Events[7].Type)
}

// TestTaskRunner_DriverNetwork asserts that a driver's network is properly
// used in services and checks.
func TestTaskRunner_DriverNetwork(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for":         "100s",
		"driver_ip":       "10.1.2.3",
		"driver_port_map": "http:80",
	}

	// Create services and checks with custom address modes to exercise
	// address detection logic
	task.Services = []*structs.Service{
		{
			Name:        "host-service",
			PortLabel:   "http",
			AddressMode: "host",
			Provider:    structs.ServiceProviderConsul,
			Checks: []*structs.ServiceCheck{
				{
					Name:        "driver-check",
					Type:        "tcp",
					PortLabel:   "1234",
					AddressMode: "driver",
				},
			},
		},
		{
			Name:        "driver-service",
			PortLabel:   "5678",
			AddressMode: "driver",
			Provider:    structs.ServiceProviderConsul,
			Checks: []*structs.ServiceCheck{
				{
					Name:      "host-check",
					Type:      "tcp",
					PortLabel: "http",
				},
				{
					Name:        "driver-label-check",
					Type:        "tcp",
					PortLabel:   "http",
					AddressMode: "driver",
				},
			},
		},
	}

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	defer cleanup()

	// Use a mock agent to test for services
	consulAgent := agentconsul.NewMockAgent(agentconsul.Features{
		Enterprise: false,
		Namespaces: false,
	})
	namespacesClient := agentconsul.NewNamespacesClient(agentconsul.NewMockNamespaces(nil), consulAgent)
	consulServices := agentconsul.NewServiceClient(consulAgent, namespacesClient, conf.Logger, true)
	defer consulServices.Shutdown()
	go consulServices.Run()

	conf.ConsulServices = consulServices
	conf.ServiceRegWrapper = wrapper.NewHandlerWrapper(conf.Logger, consulServices, nil)

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	go tr.Run()

	// Wait for the task to start
	testWaitForTaskToStart(t, tr)

	testutil.WaitForResult(func() (bool, error) {
		services, _ := consulAgent.ServicesWithFilterOpts("", nil)
		if n := len(services); n != 2 {
			return false, fmt.Errorf("expected 2 services, but found %d", n)
		}
		for _, s := range services {
			switch s.Service {
			case "host-service":
				if expected := "192.168.0.100"; s.Address != expected {
					return false, fmt.Errorf("expected host-service to have IP=%s but found %s",
						expected, s.Address)
				}
			case "driver-service":
				if expected := "10.1.2.3"; s.Address != expected {
					return false, fmt.Errorf("expected driver-service to have IP=%s but found %s",
						expected, s.Address)
				}
				if expected := 5678; s.Port != expected {
					return false, fmt.Errorf("expected driver-service to have port=%d but found %d",
						expected, s.Port)
				}
			default:
				return false, fmt.Errorf("unexpected service: %q", s.Service)
			}

		}

		checks := consulAgent.CheckRegs()
		if n := len(checks); n != 3 {
			return false, fmt.Errorf("expected 3 checks, but found %d", n)
		}
		for _, check := range checks {
			switch check.Name {
			case "driver-check":
				if expected := "10.1.2.3:1234"; check.TCP != expected {
					return false, fmt.Errorf("expected driver-check to have address %q but found %q", expected, check.TCP)
				}
			case "driver-label-check":
				if expected := "10.1.2.3:80"; check.TCP != expected {
					return false, fmt.Errorf("expected driver-label-check to have address %q but found %q", expected, check.TCP)
				}
			case "host-check":
				if expected := "192.168.0.100:"; !strings.HasPrefix(check.TCP, expected) {
					return false, fmt.Errorf("expected host-check to have address start with %q but found %q", expected, check.TCP)
				}
			default:
				return false, fmt.Errorf("unexpected check: %q", check.Name)
			}
		}

		return true, nil
	}, func(err error) {
		services, _ := consulAgent.ServicesWithFilterOpts("", nil)
		for _, s := range services {
			t.Logf(pretty.Sprint("Service: ", s))
		}
		for _, c := range consulAgent.CheckRegs() {
			t.Logf(pretty.Sprint("Check:   ", c))
		}
		require.NoError(t, err)
	})
}

// TestTaskRunner_RestartSignalTask_NotRunning asserts resilience to failures
// when a restart or signal is triggered and the task is not running.
func TestTaskRunner_RestartSignalTask_NotRunning(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "0s",
	}

	// Use vault to block the start
	task.Vault = &structs.Vault{
		Cluster:  structs.VaultDefaultCluster,
		Policies: []string{"default"},
	}

	// Control when we get a Vault token
	waitCh := make(chan struct{}, 1)
	defer close(waitCh)
	handler := func(*structs.Allocation, []string) (map[string]string, error) {
		<-waitCh
		return map[string]string{task.Name: "1234"}, nil
	}
	vc, err := vaultclient.NewMockVaultClient(structs.VaultDefaultCluster)
	must.NoError(t, err)
	vaultClient := vc.(*vaultclient.MockVaultClient)
	vaultClient.DeriveTokenFn = handler

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, vaultClient)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	go tr.Run()

	select {
	case <-tr.WaitCh():
		require.Fail(t, "unexpected exit")
	case <-time.After(1 * time.Second):
	}

	require.Equal(t, structs.TaskStatePending, tr.TaskState().State)

	// Send a signal and restart
	err = tr.Signal(structs.NewTaskEvent("don't panic"), "QUIT")
	require.EqualError(t, err, ErrTaskNotRunning.Error())

	// Send a restart
	err = tr.Restart(context.Background(), structs.NewTaskEvent("don't panic"), false)
	require.EqualError(t, err, ErrTaskNotRunning.Error())

	// Unblock and let it finish
	waitCh <- struct{}{}
	testWaitForTaskToDie(t, tr)

	// Assert the task ran and never restarted
	state := tr.TaskState()
	require.Equal(t, structs.TaskStateDead, state.State)
	require.False(t, state.Failed)
	require.Len(t, state.Events, 4, pretty.Sprint(state.Events))
	require.Equal(t, structs.TaskReceived, state.Events[0].Type)
	require.Equal(t, structs.TaskSetup, state.Events[1].Type)
	require.Equal(t, structs.TaskStarted, state.Events[2].Type)
	require.Equal(t, structs.TaskTerminated, state.Events[3].Type)
}

// TestTaskRunner_Run_RecoverableStartError asserts tasks are restarted if they
// return a recoverable error from StartTask.
func TestTaskRunner_Run_RecoverableStartError(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Config = map[string]interface{}{
		"start_error":             "driver failure",
		"start_error_recoverable": true,
	}

	// Make the restart policy retry once
	rp := &structs.RestartPolicy{
		Attempts: 1,
		Interval: 10 * time.Minute,
		Delay:    0,
		Mode:     structs.RestartPolicyModeFail,
	}
	alloc.Job.TaskGroups[0].RestartPolicy = rp
	alloc.Job.TaskGroups[0].Tasks[0].RestartPolicy = rp

	tr, _, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

	testWaitForTaskToDie(t, tr)

	state := tr.TaskState()
	require.Equal(t, structs.TaskStateDead, state.State)
	require.True(t, state.Failed)
	require.Len(t, state.Events, 6, pretty.Sprint(state.Events))
	require.Equal(t, structs.TaskReceived, state.Events[0].Type)
	require.Equal(t, structs.TaskSetup, state.Events[1].Type)
	require.Equal(t, structs.TaskDriverFailure, state.Events[2].Type)
	require.Equal(t, structs.TaskRestarting, state.Events[3].Type)
	require.Equal(t, structs.TaskDriverFailure, state.Events[4].Type)
	require.Equal(t, structs.TaskNotRestarting, state.Events[5].Type)
}

// TestTaskRunner_Template_Artifact asserts that tasks can use artifacts as templates.
func TestTaskRunner_Template_Artifact(t *testing.T) {
	ci.Parallel(t)

	ts := httptest.NewServer(http.FileServer(http.Dir(".")))
	defer ts.Close()

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	f1 := "task_runner.go"
	f2 := "test"
	task.Artifacts = []*structs.TaskArtifact{
		{GetterSource: fmt.Sprintf("%s/%s", ts.URL, f1)},
	}
	task.Templates = []*structs.Template{
		{
			SourcePath: f1,
			DestPath:   "local/test",
			ChangeMode: structs.TemplateChangeModeNoop,
		},
	}

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	go tr.Run()

	// Wait for task to run and exit
	testWaitForTaskToDie(t, tr)

	state := tr.TaskState()
	require.Equal(t, structs.TaskStateDead, state.State)
	require.True(t, state.Successful())
	require.False(t, state.Failed)

	artifactsDownloaded := false
	for _, e := range state.Events {
		if e.Type == structs.TaskDownloadingArtifacts {
			artifactsDownloaded = true
		}
	}
	assert.True(t, artifactsDownloaded, "expected artifacts downloaded events")

	// Check that both files exist.
	_, err = os.Stat(filepath.Join(conf.TaskDir.Dir, f1))
	require.NoErrorf(t, err, "%v not downloaded", f1)

	_, err = os.Stat(filepath.Join(conf.TaskDir.LocalDir, f2))
	require.NoErrorf(t, err, "%v not rendered", f2)
}

// TestTaskRunner_Template_BlockingPreStart asserts that a template
// that fails to render in PreStart can gracefully be shutdown by
// either killCtx or shutdownCtx
func TestTaskRunner_Template_BlockingPreStart(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Templates = []*structs.Template{
		{
			EmbeddedTmpl: `{{ with secret "foo/secret" }}{{ .Data.certificate }}{{ end }}`,
			DestPath:     "local/test",
			ChangeMode:   structs.TemplateChangeModeNoop,
		},
	}

	task.Vault = &structs.Vault{
		Cluster:  structs.VaultDefaultCluster,
		Policies: []string{"default"},
	}

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	go tr.Run()
	defer tr.Shutdown()

	testutil.WaitForResult(func() (bool, error) {
		ts := tr.TaskState()

		if len(ts.Events) == 0 {
			return false, fmt.Errorf("no events yet")
		}

		for _, e := range ts.Events {
			if e.Type == "Template" && strings.Contains(e.DisplayMessage, "vault.read(foo/secret)") {
				return true, nil
			}
		}

		return false, fmt.Errorf("no missing vault secret template event yet: %#v", ts.Events)

	}, func(err error) {
		require.NoError(t, err)
	})

	shutdown := func() <-chan bool {
		finished := make(chan bool)
		go func() {
			tr.Shutdown()
			finished <- true
		}()

		return finished
	}

	select {
	case <-shutdown():
		// it shut down like it should have
	case <-time.After(10 * time.Second):
		require.Fail(t, "timeout shutting down task")
	}
}

func TestTaskRunner_TemplateWorkloadIdentity(t *testing.T) {
	ci.Parallel(t)

	expectedConsulValue := "consul-value"
	consulKVResp := fmt.Sprintf(`
[
    {
        "LockIndex": 0,
        "Key": "consul-key",
        "Flags": 0,
        "Value": "%s",
        "CreateIndex": 57,
        "ModifyIndex": 57
    }
]
`, base64.StdEncoding.EncodeToString([]byte(expectedConsulValue)))

	expectedVaultSecret := "vault-secret"
	vaultSecretResp := fmt.Sprintf(`
{
  "data": {
    "data": {
      "secret": "%s"
    },
    "metadata": {
      "created_time": "2023-10-18T15:58:29.65137Z",
      "custom_metadata": null,
      "deletion_time": "",
      "destroyed": false,
      "version": 1
    }
  }
}`, expectedVaultSecret)

	// Start a test server for Consul and Vault.
	vaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, vaultSecretResp)
	}))
	t.Cleanup(vaultServer.Close)

	firstConsulRequest := &atomic.Bool{}
	firstConsulRequest.Store(true)
	consulServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !firstConsulRequest.Load() {
			// Simulate a blocking query to avoid a tight loop in
			// consul-template.
			var wait time.Duration
			if waitStr := r.FormValue("wait"); waitStr != "" {
				wait, _ = time.ParseDuration(waitStr)
			}

			if wait != 0 {
				doneCh := make(chan struct{})
				t.Cleanup(func() { close(doneCh) })

				timer, stop := helper.NewSafeTimer(wait)
				t.Cleanup(stop)

				select {
				case <-timer.C:
				case <-doneCh:
					return
				}
			}
		} else {
			// Send response immediately if this is the first request.
			firstConsulRequest.Store(true)
		}

		// Return an index so consul-template knows that the blocking query
		// didn't timeout on first run.
		// https://github.com/hashicorp/consul-template/blob/2d2654ffe96210db43306922aaefbb730a8e07f9/watch/view.go#L267-L272
		w.Header().Set("X-Consul-Index", "57")
		fmt.Fprintln(w, consulKVResp)
	}))
	t.Cleanup(consulServer.Close)

	// Create allocation with a template that reads from Consul and Vault.
	alloc := mock.BatchAlloc()
	alloc.Job.TaskGroups[0].Count = 1
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "2s",
	}
	task.Consul = &structs.Consul{
		Cluster: structs.ConsulDefaultCluster,
	}
	task.Vault = &structs.Vault{
		Cluster: structs.VaultDefaultCluster,
	}
	task.Identities = []*structs.WorkloadIdentity{
		{Name: task.Consul.IdentityName()},
		{Name: task.Vault.IdentityName()},
	}
	task.Templates = []*structs.Template{
		{
			EmbeddedTmpl: `
{{key "consul-key"}}
{{with secret "secret/data/vault-key"}}{{.Data.data.secret}}{{end}}
`,
			DestPath: "local/out.txt",
		},
	}

	// Create task runner with a Consul and Vault cluster configured.
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	conf.ClientConfig.ConsulConfigs = map[string]*structsc.ConsulConfig{
		structs.ConsulDefaultCluster: {
			Addr: consulServer.URL,
		},
	}
	conf.ClientConfig.VaultConfigs = map[string]*structsc.VaultConfig{
		structs.VaultDefaultCluster: {
			Enabled: pointer.Of(true),
			Addr:    vaultServer.URL,
		},
	}
	conf.AllocHookResources.SetConsulTokens(map[string]map[string]*consulapi.ACLToken{
		structs.ConsulDefaultCluster: {
			task.Consul.IdentityName() + "/web": {SecretID: "consul-task-token"},
		},
	})
	t.Cleanup(cleanup)

	tr, err := NewTaskRunner(conf)
	must.NoError(t, err)

	// Run the task runner.
	go tr.Run()
	t.Cleanup(func() {
		tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	})

	// Wait for task to complete.
	testWaitForTaskToStart(t, tr)

	// Verify template was rendered with the expected content.
	data, err := os.ReadFile(path.Join(conf.TaskDir.LocalDir, "out.txt"))
	must.NoError(t, err)

	renderedTmpl := string(data)
	must.StrContains(t, renderedTmpl, expectedConsulValue)
	must.StrContains(t, renderedTmpl, expectedVaultSecret)
}

// TestTaskRunner_Template_NewVaultToken asserts that a new vault token is
// created when rendering template and that it is revoked on alloc completion
func TestTaskRunner_Template_NewVaultToken(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Templates = []*structs.Template{
		{
			EmbeddedTmpl: `{{key "foo"}}`,
			DestPath:     "local/test",
			ChangeMode:   structs.TemplateChangeModeNoop,
		},
	}
	task.Vault = &structs.Vault{
		Cluster:  structs.VaultDefaultCluster,
		Policies: []string{"default"},
	}

	vc, err := vaultclient.NewMockVaultClient(structs.VaultDefaultCluster)
	must.NoError(t, err)
	vaultClient := vc.(*vaultclient.MockVaultClient)

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, vaultClient)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	go tr.Run()

	// Wait for a Vault token
	var token string
	testutil.WaitForResult(func() (bool, error) {
		token = tr.getVaultToken()

		if token == "" {
			return false, fmt.Errorf("No Vault token")
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	renewalCh, ok := vaultClient.RenewTokens()[token]
	require.True(t, ok, "no renewal channel for token")

	renewalCh <- fmt.Errorf("Test killing")
	close(renewalCh)

	var token2 string
	testutil.WaitForResult(func() (bool, error) {
		token2 = tr.getVaultToken()

		if token2 == "" {
			return false, fmt.Errorf("No Vault token")
		}

		if token2 == token {
			return false, fmt.Errorf("token wasn't recreated")
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// Check the token was revoked
	testutil.WaitForResult(func() (bool, error) {
		if len(vaultClient.StoppedTokens()) != 1 {
			return false, fmt.Errorf("Expected a stopped token: %v", vaultClient.StoppedTokens())
		}

		if a := vaultClient.StoppedTokens()[0]; a != token {
			return false, fmt.Errorf("got stopped token %q; want %q", a, token)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

}

// TestTaskRunner_VaultManager_Restart asserts that the alloc is restarted when the alloc
// derived vault token expires, when task is configured with Restart change mode
func TestTaskRunner_VaultManager_Restart(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}
	task.Vault = &structs.Vault{
		Cluster:    structs.VaultDefaultCluster,
		Policies:   []string{"default"},
		ChangeMode: structs.VaultChangeModeRestart,
	}

	vc, err := vaultclient.NewMockVaultClient(structs.VaultDefaultCluster)
	vaultClient := vc.(*vaultclient.MockVaultClient)
	must.NoError(t, err)

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, vaultClient)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	go tr.Run()

	testWaitForTaskToStart(t, tr)

	tr.vaultTokenLock.Lock()
	token := tr.vaultToken
	tr.vaultTokenLock.Unlock()

	require.NotEmpty(t, token)

	renewalCh, ok := vaultClient.RenewTokens()[token]
	require.True(t, ok, "no renewal channel for token")

	renewalCh <- fmt.Errorf("Test killing")
	close(renewalCh)

	testutil.WaitForResult(func() (bool, error) {
		state := tr.TaskState()

		if len(state.Events) == 0 {
			return false, fmt.Errorf("no events yet")
		}

		foundRestartSignal, foundRestarting := false, false
		for _, e := range state.Events {
			switch e.Type {
			case structs.TaskRestartSignal:
				foundRestartSignal = true
			case structs.TaskRestarting:
				foundRestarting = true
			}
		}

		if !foundRestartSignal {
			return false, fmt.Errorf("no restart signal event yet: %#v", state.Events)
		}

		if !foundRestarting {
			return false, fmt.Errorf("no restarting event yet: %#v", state.Events)
		}

		lastEvent := state.Events[len(state.Events)-1]
		if lastEvent.Type != structs.TaskStarted {
			return false, fmt.Errorf("expected last event to be task starting but was %#v", lastEvent)
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

// TestTaskRunner_VaultManager_Signal asserts that the alloc is signalled when the alloc
// derived vault token expires, when task is configured with signal change mode
func TestTaskRunner_VaultManager_Signal(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}
	task.Vault = &structs.Vault{
		Cluster:      structs.VaultDefaultCluster,
		Policies:     []string{"default"},
		ChangeMode:   structs.VaultChangeModeSignal,
		ChangeSignal: "SIGUSR1",
	}
	vc, err := vaultclient.NewMockVaultClient(structs.VaultDefaultCluster)
	must.NoError(t, err)
	vaultClient := vc.(*vaultclient.MockVaultClient)

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, vaultClient)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	go tr.Run()

	testWaitForTaskToStart(t, tr)

	tr.vaultTokenLock.Lock()
	token := tr.vaultToken
	tr.vaultTokenLock.Unlock()

	require.NotEmpty(t, token)

	renewalCh, ok := vaultClient.RenewTokens()[token]
	require.True(t, ok, "no renewal channel for token")

	renewalCh <- fmt.Errorf("Test killing")
	close(renewalCh)

	testutil.WaitForResult(func() (bool, error) {
		state := tr.TaskState()

		if len(state.Events) == 0 {
			return false, fmt.Errorf("no events yet")
		}

		foundSignaling := false
		for _, e := range state.Events {
			if e.Type == structs.TaskSignaling {
				foundSignaling = true
			}
		}

		if !foundSignaling {
			return false, fmt.Errorf("no signaling event yet: %#v", state.Events)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

}

// TestTaskRunner_UnregisterConsul_Retries asserts a task is unregistered from
// Consul when waiting to be retried.
func TestTaskRunner_UnregisterConsul_Retries(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	// Make the restart policy try one ctx.update
	rp := &structs.RestartPolicy{
		Attempts: 1,
		Interval: 10 * time.Minute,
		Delay:    time.Nanosecond,
		Mode:     structs.RestartPolicyModeFail,
	}
	alloc.Job.TaskGroups[0].RestartPolicy = rp
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.RestartPolicy = rp
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "1",
		"run_for":   "1ns",
	}

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	defer cleanup()

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	go tr.Run()

	testWaitForTaskToDie(t, tr)

	state := tr.TaskState()
	require.Equal(t, structs.TaskStateDead, state.State)

	consulServices := conf.ConsulServices.(*regMock.ServiceRegistrationHandler)
	consulOps := consulServices.GetOps()
	require.Len(t, consulOps, 4)

	// Initial add
	require.Equal(t, "add", consulOps[0].Op)

	// Removing entries on first exit
	require.Equal(t, "remove", consulOps[1].Op)

	// Second add on retry
	require.Equal(t, "add", consulOps[2].Op)

	// Removing entries on retry
	require.Equal(t, "remove", consulOps[3].Op)
}

// testWaitForTaskToStart waits for the task to be running or fails the test
func testWaitForTaskToStart(t *testing.T, tr *TaskRunner) {
	testutil.WaitForResult(func() (bool, error) {
		ts := tr.TaskState()
		return ts.State == structs.TaskStateRunning, fmt.Errorf("expected task to be running, got %v", ts.State)
	}, func(err error) {
		require.NoError(t, err)
	})
}

// testWaitForTaskToDie waits for the task to die or fails the test
func testWaitForTaskToDie(t *testing.T, tr *TaskRunner) {
	testutil.WaitForResult(func() (bool, error) {
		ts := tr.TaskState()
		return ts.State == structs.TaskStateDead, fmt.Errorf("expected task to be dead, got %v", ts.State)
	}, func(err error) {
		require.NoError(t, err)
	})
}

// TestTaskRunner_BaseLabels tests that the base labels for the task metrics
// are set appropriately.
func TestTaskRunner_BaseLabels(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	alloc := mock.BatchAlloc()
	alloc.Namespace = "not-default"
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "raw_exec"
	task.Config = map[string]interface{}{
		"command": "whoami",
	}

	config, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	defer cleanup()

	tr, err := NewTaskRunner(config)
	require.NoError(err)

	labels := map[string]string{}
	for _, e := range tr.baseLabels {
		labels[e.Name] = e.Value
	}
	require.Equal(alloc.Job.Name, labels["job"])
	require.Equal(alloc.TaskGroup, labels["task_group"])
	require.Equal(task.Name, labels["task"])
	require.Equal(alloc.ID, labels["alloc_id"])
	require.Equal(alloc.Namespace, labels["namespace"])
}

// TestTaskRunner_BaseLabels_IncludesAllocMetadata tests that the base labels include
// the allocation metadata fields using the provided allowed list of keys
func TestTaskRunner_BaseLabels_IncludesAllocMetadata(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	alloc.Namespace = "not-default"
	job := alloc.Job
	job.Meta = map[string]string{"owner": "HashiCorp", "my-key": "my-value", "some_dynamic_value": "now()"}
	task := job.TaskGroups[0].Tasks[0]
	task.Driver = "raw_exec"
	task.Config = map[string]interface{}{
		"command": "whoami",
	}

	trConfig, cleanup := testTaskRunnerConfig(t, alloc, task.Name, nil)
	defer cleanup()

	trConfig.ClientConfig.IncludeAllocMetadataInMetrics = true
	trConfig.ClientConfig.AllowedMetadataKeysInMetrics = []string{"owner", "my-key"}

	tr, err := NewTaskRunner(trConfig)
	must.NoError(t, err)

	labels := map[string]string{}
	for _, e := range tr.baseLabels {
		labels[e.Name] = e.Value
	}

	must.Eq(t, "HashiCorp", labels["owner"])
	must.Eq(t, "my-value", labels["my_key"])
	must.MapNotContainsKey(t, labels, "some_dynamic_value")
}

// TestTaskRunner_IdentityHook_Enabled asserts that the identity hook exposes a
// workload identity to a task.
func TestTaskRunner_IdentityHook_Enabled(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	// Fake an identity and expose it to the task
	alloc.SignedIdentities = map[string]string{
		task.Name: "foo",
	}
	task.Identity = &structs.WorkloadIdentity{
		Env:  true,
		File: true,
	}
	task.Identities = []*structs.WorkloadIdentity{
		{
			Name:     "consul",
			Audience: []string{"a", "b"},
			Env:      true,
		},
		{
			Name: "vault",
			File: true,
		},
	}

	tr, _, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

	testWaitForTaskToDie(t, tr)

	// Assert tokens were written to the filesystem
	tokenBytes, err := os.ReadFile(filepath.Join(tr.taskDir.SecretsDir, "nomad_token"))
	must.NoError(t, err)
	must.Eq(t, "foo", string(tokenBytes))

	tokenBytes, err = os.ReadFile(filepath.Join(tr.taskDir.SecretsDir, "nomad_consul.jwt"))
	must.ErrorIs(t, err, os.ErrNotExist)

	tokenBytes, err = os.ReadFile(filepath.Join(tr.taskDir.SecretsDir, "nomad_vault.jwt"))
	must.NoError(t, err)
	must.StrContains(t, string(tokenBytes), ".")

	// Assert tokens are built into the task env
	taskEnv := tr.envBuilder.Build()
	must.Eq(t, "foo", taskEnv.EnvMap["NOMAD_TOKEN"])
	must.StrContains(t, taskEnv.EnvMap["NOMAD_TOKEN_consul"], ".")
	must.MapNotContainsKey(t, taskEnv.EnvMap, "NOMAD_TOKEN_vault")
}

// TestTaskRunner_IdentityHook_Disabled asserts that the identity hook does not
// expose a workload identity to a task by default.
func TestTaskRunner_IdentityHook_Disabled(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	// Fake an identity but don't expose it to the task
	alloc.SignedIdentities = map[string]string{
		task.Name: "foo",
	}
	task.Identity = nil

	tr, _, cleanup := runTestTaskRunner(t, alloc, task.Name)
	defer cleanup()

	testWaitForTaskToDie(t, tr)

	// Assert the token was written to the filesystem
	_, err := os.ReadFile(filepath.Join(tr.taskDir.SecretsDir, "nomad_token"))
	must.Error(t, err)

	// Assert the token is built into the task env
	taskEnv := tr.envBuilder.Build()
	must.MapNotContainsKey(t, taskEnv.EnvMap, "NOMAD_TOKEN")
}

func TestTaskRunner_AllocNetworkStatus(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{"run_for": "2s"}

	groupNetworks := []*structs.NetworkResource{{
		Device: "eth0",
		IP:     "192.168.0.100",
		DNS: &structs.DNSConfig{
			Servers:  []string{"1.1.1.1", "8.8.8.8"},
			Searches: []string{"test.local"},
			Options:  []string{"ndots:1"},
		},
		ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
		DynamicPorts:  []structs.Port{{Label: "http", Value: 9876}},
	}}

	groupNetworksWithoutDNS := []*structs.NetworkResource{{
		Device:        "eth0",
		IP:            "192.168.0.100",
		ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
		DynamicPorts:  []structs.Port{{Label: "http", Value: 9876}},
	}}

	testCases := []struct {
		name     string
		networks []*structs.NetworkResource
		fromCNI  *structs.DNSConfig
		expect   *drivers.DNSConfig
	}{
		{
			name:     "task with group networking overrides CNI",
			networks: groupNetworks,
			fromCNI: &structs.DNSConfig{
				Servers:  []string{"10.37.105.17"},
				Searches: []string{"node.consul"},
				Options:  []string{"ndots:2", "edns0"},
			},
			expect: &drivers.DNSConfig{
				Servers:  []string{"1.1.1.1", "8.8.8.8"},
				Searches: []string{"test.local"},
				Options:  []string{"ndots:1"},
			},
		},
		{
			name: "task with CNI alone",
			fromCNI: &structs.DNSConfig{
				Servers:  []string{"10.37.105.17"},
				Searches: []string{"node.consul"},
				Options:  []string{"ndots:2", "edns0"},
			},
			expect: &drivers.DNSConfig{
				Servers:  []string{"10.37.105.17"},
				Searches: []string{"node.consul"},
				Options:  []string{"ndots:2", "edns0"},
			},
		},
		{
			name:     "task with group networking alone wth DNS",
			networks: groupNetworks,
			fromCNI:  nil,
			expect: &drivers.DNSConfig{
				Servers:  []string{"1.1.1.1", "8.8.8.8"},
				Searches: []string{"test.local"},
				Options:  []string{"ndots:1"},
			},
		},
		{
			name:     "task with group networking and no CNI dns",
			networks: groupNetworksWithoutDNS,
			fromCNI:  &structs.DNSConfig{},
			expect:   &drivers.DNSConfig{},
		},
		{
			name:    "task without group networking or CNI",
			fromCNI: nil,
			expect:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			testAlloc := alloc.Copy()
			testAlloc.AllocatedResources.Shared.Networks = tc.networks

			conf, cleanup := testTaskRunnerConfig(t, testAlloc, task.Name, nil)
			t.Cleanup(cleanup)

			// note this will never actually be set if we don't have group/CNI
			// networking, but it's a good validation no-group/CNI code path
			conf.AllocHookResources.SetAllocNetworkStatus(&structs.AllocNetworkStatus{
				InterfaceName: "",
				Address:       "",
				DNS:           tc.fromCNI,
			})

			tr, err := NewTaskRunner(conf)
			must.NoError(t, err)

			// Run the task runner.
			go tr.Run()
			t.Cleanup(func() {
				tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
			})

			// Wait for task to complete.
			testWaitForTaskToStart(t, tr)

			tr.stateLock.RLock()
			t.Cleanup(tr.stateLock.RUnlock)

			must.Eq(t, tc.expect, tr.localState.TaskHandle.Config.DNS)
		})
	}
}
