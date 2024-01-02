// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package allocrunner

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	regMock "github.com/hashicorp/nomad/client/serviceregistration/mock"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

// TestAllocRunner_Restore_RunningTerminal asserts that restoring a terminal
// alloc with a running task properly kills the running the task. This is meant
// to simulate a Nomad agent crash after receiving an updated alloc with
// DesiredStatus=Stop, persisting the update, but crashing before terminating
// the task.
func TestAllocRunner_Restore_RunningTerminal(t *testing.T) {
	ci.Parallel(t)

	// 1. Run task
	// 2. Shutdown alloc runner
	// 3. Set alloc.desiredstatus=false
	// 4. Start new alloc runner
	// 5. Assert task and logmon are cleaned up

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Services = []*structs.Service{
		{
			Name:      "foo",
			PortLabel: "8888",
			Provider:  structs.ServiceProviderConsul,
		},
	}
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1h",
	}

	conf, cleanup := testAllocRunnerConfig(t, alloc.Copy())
	defer cleanup()

	// Maintain state for subsequent run
	conf.StateDB = state.NewMemDB(conf.Logger)

	// Start and wait for task to be running
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)
	go ar.Run()
	defer destroy(ar)

	testutil.WaitForResult(func() (bool, error) {
		s := ar.AllocState()
		return s.ClientStatus == structs.AllocClientStatusRunning, fmt.Errorf("expected running, got %s", s.ClientStatus)
	}, func(err error) {
		require.NoError(t, err)
	})

	// Shutdown the AR and manually change the state to mimic a crash where
	// a stopped alloc update is received, but Nomad crashes before
	// stopping the alloc.
	ar.Shutdown()
	select {
	case <-ar.ShutdownCh():
	case <-time.After(30 * time.Second):
		require.Fail(t, "AR took too long to exit")
	}

	// Assert logmon is still running. This is a super ugly hack that pulls
	// logmon's PID out of its reattach config, but it does properly ensure
	// logmon gets cleaned up.
	ls, _, err := conf.StateDB.GetTaskRunnerState(alloc.ID, task.Name)
	require.NoError(t, err)
	require.NotNil(t, ls)

	logmonReattach := struct {
		Pid int
	}{}
	err = json.Unmarshal([]byte(ls.Hooks["logmon"].Data["reattach_config"]), &logmonReattach)
	require.NoError(t, err)

	logmonProc, _ := os.FindProcess(logmonReattach.Pid)
	require.NoError(t, logmonProc.Signal(syscall.Signal(0)))

	// Fake alloc terminal during Restore()
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	alloc.ModifyIndex++
	alloc.AllocModifyIndex++

	// Start a new alloc runner and assert it gets stopped
	conf2, cleanup2 := testAllocRunnerConfig(t, alloc)
	defer cleanup2()

	// Use original statedb to maintain hook state
	conf2.StateDB = conf.StateDB

	// Restore, start, and wait for task to be killed
	ar2Iface, err := NewAllocRunner(conf2)
	must.NoError(t, err)
	ar2 := ar2Iface.(*allocRunner)

	require.NoError(t, ar2.Restore())

	go ar2.Run()
	defer destroy(ar2)

	select {
	case <-ar2.WaitCh():
	case <-time.After(30 * time.Second):
	}

	// Assert logmon was cleaned up
	require.Error(t, logmonProc.Signal(syscall.Signal(0)))

	// Assert consul was cleaned up:
	//   1 removal during prekill
	//    - removal during exited is de-duped due to prekill
	//    - removal during stop is de-duped due to prekill
	//   1 removal group during stop
	consulOps := conf2.Consul.(*regMock.ServiceRegistrationHandler).GetOps()
	require.Len(t, consulOps, 2)
	for _, op := range consulOps {
		require.Equal(t, "remove", op.Op)
	}

	// Assert terminated task event was emitted
	events := ar2.AllocState().TaskStates[task.Name].Events
	require.Len(t, events, 4)
	require.Equal(t, events[0].Type, structs.TaskReceived)
	require.Equal(t, events[1].Type, structs.TaskSetup)
	require.Equal(t, events[2].Type, structs.TaskStarted)
	require.Equal(t, events[3].Type, structs.TaskTerminated)
}

// TestAllocRunner_Restore_CompletedBatch asserts that restoring a completed
// batch alloc doesn't run it again
func TestAllocRunner_Restore_CompletedBatch(t *testing.T) {
	ci.Parallel(t)

	// 1. Run task and wait for it to complete
	// 2. Start new alloc runner
	// 3. Assert task didn't run again

	alloc := mock.Alloc()
	alloc.Job.Type = structs.JobTypeBatch
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "2ms",
	}

	conf, cleanup := testAllocRunnerConfig(t, alloc.Copy())
	defer cleanup()

	// Maintain state for subsequent run
	conf.StateDB = state.NewMemDB(conf.Logger)

	// Start and wait for task to be running
	arIface, err := NewAllocRunner(conf)
	must.NoError(t, err)
	ar := arIface.(*allocRunner)
	go ar.Run()
	defer destroy(ar)

	testutil.WaitForResult(func() (bool, error) {
		s := ar.AllocState()
		if s.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("expected complete, got %s", s.ClientStatus)
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// once job finishes, it shouldn't run again
	require.False(t, ar.shouldRun())
	initialRunEvents := ar.AllocState().TaskStates[task.Name].Events
	require.Len(t, initialRunEvents, 4)

	ls, ts, err := conf.StateDB.GetTaskRunnerState(alloc.ID, task.Name)
	require.NoError(t, err)
	require.NotNil(t, ls)
	require.Equal(t, structs.TaskStateDead, ts.State)

	// Start a new alloc runner and assert it gets stopped
	conf2, cleanup2 := testAllocRunnerConfig(t, alloc)
	defer cleanup2()

	// Use original statedb to maintain hook state
	conf2.StateDB = conf.StateDB

	// Restore, start, and wait for task to be killed
	ar2Iface, err := NewAllocRunner(conf2)
	must.NoError(t, err)
	ar2 := ar2Iface.(*allocRunner)
	must.NoError(t, ar2.Restore())

	go ar2.Run()
	defer destroy(ar2)

	// AR waitCh must be open as the task waits for a possible alloc restart.
	select {
	case <-ar2.WaitCh():
		require.Fail(t, "alloc.waitCh was closed")
	default:
	}

	// TR waitCh must be open too!
	select {
	case <-ar2.tasks[task.Name].WaitCh():
		require.Fail(t, "tr.waitCh was closed")
	default:
	}

	// Assert that events are unmodified, which they would if task re-run
	events := ar2.AllocState().TaskStates[task.Name].Events
	require.Equal(t, initialRunEvents, events)
}

// TestAllocRunner_PreStartFailuresLeadToFailed asserts that if an alloc
// prestart hooks failed, then the alloc and subsequent tasks transition
// to failed state
func TestAllocRunner_PreStartFailuresLeadToFailed(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	alloc.Job.Type = structs.JobTypeBatch
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "2ms",
	}
	rp := &structs.RestartPolicy{Attempts: 0}
	alloc.Job.TaskGroups[0].RestartPolicy = rp
	task.RestartPolicy = rp

	conf, cleanup := testAllocRunnerConfig(t, alloc.Copy())
	defer cleanup()

	// Maintain state for subsequent run
	conf.StateDB = state.NewMemDB(conf.Logger)

	// Start and wait for task to be running
	arIface, err := NewAllocRunner(conf)
	must.NoError(t, err)
	ar := arIface.(*allocRunner)
	ar.runnerHooks = append(ar.runnerHooks, &allocFailingPrestartHook{})

	go ar.Run()
	defer destroy(ar)

	select {
	case <-ar.WaitCh():
	case <-time.After(10 * time.Second):
		require.Fail(t, "alloc.waitCh wasn't closed")
	}

	testutil.WaitForResult(func() (bool, error) {
		s := ar.AllocState()
		if s.ClientStatus != structs.AllocClientStatusFailed {
			return false, fmt.Errorf("expected complete, got %s", s.ClientStatus)
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// once job finishes, it shouldn't run again
	require.False(t, ar.shouldRun())
	initialRunEvents := ar.AllocState().TaskStates[task.Name].Events
	require.Len(t, initialRunEvents, 2)

	ls, ts, err := conf.StateDB.GetTaskRunnerState(alloc.ID, task.Name)
	require.NoError(t, err)
	require.NotNil(t, ls)
	require.NotNil(t, ts)
	require.Equal(t, structs.TaskStateDead, ts.State)
	require.True(t, ts.Failed)

	// TR waitCh must be closed too!
	select {
	case <-ar.tasks[task.Name].WaitCh():
	case <-time.After(10 * time.Second):
		require.Fail(t, "tr.waitCh wasn't closed")
	}
}

type allocFailingPrestartHook struct{}

func (*allocFailingPrestartHook) Name() string { return "failing_prestart" }

func (*allocFailingPrestartHook) Prerun() error {
	return fmt.Errorf("failing prestart hooks")
}
