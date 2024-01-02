// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/serviceregistration"
	regMock "github.com/hashicorp/nomad/client/serviceregistration/mock"
	"github.com/hashicorp/nomad/client/serviceregistration/wrapper"
	"github.com/hashicorp/nomad/client/taskenv"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func newScriptMock(hb TTLUpdater, exec interfaces.ScriptExecutor, logger hclog.Logger, interval, timeout time.Duration) *scriptCheck {
	script := newScriptCheck(&scriptCheckConfig{
		allocID:   "allocid",
		taskName:  "testtask",
		serviceID: "serviceid",
		check: &structs.ServiceCheck{
			Interval: interval,
			Timeout:  timeout,
		},
		ttlUpdater: hb,
		driverExec: exec,
		taskEnv:    &taskenv.TaskEnv{},
		logger:     logger,
		shutdownCh: nil,
	})
	script.callback = newScriptCheckCallback(script)
	script.lastCheckOk = true
	return script
}

// fakeHeartbeater implements the TTLUpdater interface to allow mocking out
// Consul in script executor tests.
type fakeHeartbeater struct {
	heartbeats chan heartbeat
}

func (f *fakeHeartbeater) UpdateTTL(checkID, namespace, output, status string) error {
	f.heartbeats <- heartbeat{checkID: checkID, output: output, status: status}
	return nil
}

func newFakeHeartbeater() *fakeHeartbeater {
	return &fakeHeartbeater{heartbeats: make(chan heartbeat)}
}

type heartbeat struct {
	checkID string
	output  string
	status  string
}

// TestScript_Exec_Cancel asserts cancelling a script check shortcircuits
// any running scripts.
func TestScript_Exec_Cancel(t *testing.T) {
	ci.Parallel(t)

	exec, cancel := newBlockingScriptExec()
	defer cancel()

	logger := testlog.HCLogger(t)
	script := newScriptMock(nil, // TTLUpdater should never be called
		exec, logger, time.Hour, time.Hour)

	handle := script.run()
	<-exec.running  // wait until Exec is called
	handle.cancel() // cancel now that we're blocked in exec

	select {
	case <-handle.wait():
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}

	// The underlying ScriptExecutor (newBlockScriptExec) *cannot* be
	// canceled. Only a wrapper around it obeys the context cancelation.
	require.NotEqual(t, atomic.LoadInt32(&exec.exited), 1,
		"expected script executor to still be running after timeout")
}

// TestScript_Exec_TimeoutBasic asserts a script will be killed when the
// timeout is reached.
func TestScript_Exec_TimeoutBasic(t *testing.T) {
	ci.Parallel(t)
	exec, cancel := newBlockingScriptExec()
	defer cancel()

	logger := testlog.HCLogger(t)
	hb := newFakeHeartbeater()
	script := newScriptMock(hb, exec, logger, time.Hour, time.Second)

	handle := script.run()
	defer handle.cancel() // cleanup
	<-exec.running        // wait until Exec is called

	// Check for UpdateTTL call
	select {
	case update := <-hb.heartbeats:
		require.Equal(t, update.output, context.DeadlineExceeded.Error())
		require.Equal(t, update.status, api.HealthCritical)
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}

	// The underlying ScriptExecutor (newBlockScriptExec) *cannot* be
	// canceled. Only a wrapper around it obeys the context cancelation.
	require.NotEqual(t, atomic.LoadInt32(&exec.exited), 1,
		"expected script executor to still be running after timeout")

	// Cancel and watch for exit
	handle.cancel()
	select {
	case <-handle.wait(): // ok!
	case update := <-hb.heartbeats:
		t.Errorf("unexpected UpdateTTL call on exit with status=%q", update)
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}
}

// TestScript_Exec_TimeoutCritical asserts a script will be killed when
// the timeout is reached and always set a critical status regardless of what
// Exec returns.
func TestScript_Exec_TimeoutCritical(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)
	hb := newFakeHeartbeater()
	script := newScriptMock(hb, sleeperExec{}, logger, time.Hour, time.Nanosecond)

	handle := script.run()
	defer handle.cancel() // cleanup

	// Check for UpdateTTL call
	select {
	case update := <-hb.heartbeats:
		require.Equal(t, update.output, context.DeadlineExceeded.Error())
		require.Equal(t, update.status, api.HealthCritical)
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to timeout")
	}
}

// TestScript_Exec_Shutdown asserts a script will be executed once more
// when told to shutdown.
func TestScript_Exec_Shutdown(t *testing.T) {
	ci.Parallel(t)

	shutdown := make(chan struct{})
	exec := newSimpleExec(0, nil)
	logger := testlog.HCLogger(t)
	hb := newFakeHeartbeater()
	script := newScriptMock(hb, exec, logger, time.Hour, 3*time.Second)
	script.shutdownCh = shutdown

	handle := script.run()
	defer handle.cancel() // cleanup
	close(shutdown)       // tell scriptCheck to exit

	select {
	case update := <-hb.heartbeats:
		require.Equal(t, update.output, "code=0 err=<nil>")
		require.Equal(t, update.status, api.HealthPassing)
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}

	select {
	case <-handle.wait(): // ok!
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for script check to exit")
	}
}

// TestScript_Exec_Codes asserts script exit codes are translated to their
// corresponding Consul health check status.
func TestScript_Exec_Codes(t *testing.T) {
	ci.Parallel(t)

	exec := newScriptedExec([]execResult{
		{[]byte("output"), 1, nil},
		{[]byte("output"), 0, nil},
		{[]byte("output"), 0, context.DeadlineExceeded},
		{[]byte("output"), 0, nil},
		{[]byte("<ignored output>"), 2, fmt.Errorf("some error")},
		{[]byte("output"), 0, nil},
		{[]byte("error9000"), 9000, nil},
	})
	logger := testlog.HCLogger(t)
	hb := newFakeHeartbeater()
	script := newScriptMock(
		hb, exec, logger, time.Nanosecond, 3*time.Second)

	handle := script.run()
	defer handle.cancel() // cleanup
	deadline := time.After(3 * time.Second)

	expected := []heartbeat{
		{script.id, "output", api.HealthWarning},
		{script.id, "output", api.HealthPassing},
		{script.id, context.DeadlineExceeded.Error(), api.HealthCritical},
		{script.id, "output", api.HealthPassing},
		{script.id, "some error", api.HealthCritical},
		{script.id, "output", api.HealthPassing},
		{script.id, "error9000", api.HealthCritical},
	}

	for i := 0; i <= 6; i++ {
		select {
		case update := <-hb.heartbeats:
			require.Equal(t, update, expected[i],
				"expected update %d to be '%s' but received '%s'",
				i, expected[i], update)
		case <-deadline:
			t.Fatalf("timed out waiting for all script checks to finish")
		}
	}
}

// TestScript_TaskEnvInterpolation asserts that script check hooks are
// interpolated in the same way that services are
func TestScript_TaskEnvInterpolation(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	consulClient := regMock.NewServiceRegistrationHandler(logger)
	regWrap := wrapper.NewHandlerWrapper(logger, consulClient, nil)
	exec, cancel := newBlockingScriptExec()
	defer cancel()

	alloc := mock.ConnectAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	task.Services[0].Name = "${NOMAD_JOB_NAME}-${TASK}-${SVC_NAME}"
	task.Services[0].Checks[0].Name = "${NOMAD_JOB_NAME}-${SVC_NAME}-check"
	alloc.Job.Canonicalize() // need to re-canonicalize b/c the mock already did it

	env := taskenv.NewBuilder(mock.Node(), alloc, task, "global").SetHookEnv(
		"script_check",
		map[string]string{"SVC_NAME": "frontend"}).Build()

	svcHook := newServiceHook(serviceHookConfig{
		alloc:             alloc,
		task:              task,
		serviceRegWrapper: regWrap,
		logger:            logger,
	})
	// emulate prestart having been fired
	svcHook.taskEnv = env

	scHook := newScriptCheckHook(scriptCheckHookConfig{
		alloc:        alloc,
		task:         task,
		consul:       consulClient,
		logger:       logger,
		shutdownWait: time.Hour, // TTLUpdater will never be called
	})
	// emulate prestart having been fired
	scHook.taskEnv = env
	scHook.driverExec = exec

	workload := svcHook.getWorkloadServices()
	must.Eq(t, "web", workload.AllocInfo.Group)

	expectedSvc := workload.Services[0]
	expected := agentconsul.MakeCheckID(serviceregistration.MakeAllocServiceID(
		alloc.ID, task.Name, expectedSvc), expectedSvc.Checks[0])

	actual := scHook.newScriptChecks()
	check, ok := actual[expected]
	must.True(t, ok)
	must.Eq(t, "my-job-frontend-check", check.check.Name)

	// emulate an update
	env = taskenv.NewBuilder(mock.Node(), alloc, task, "global").SetHookEnv(
		"script_check",
		map[string]string{"SVC_NAME": "backend"}).Build()
	scHook.taskEnv = env
	svcHook.taskEnv = env

	expectedSvc = svcHook.getWorkloadServices().Services[0]
	expected = agentconsul.MakeCheckID(serviceregistration.MakeAllocServiceID(
		alloc.ID, task.Name, expectedSvc), expectedSvc.Checks[0])

	actual = scHook.newScriptChecks()
	check, ok = actual[expected]
	must.True(t, ok)
	must.Eq(t, "my-job-backend-check", check.check.Name)
}

func TestScript_associated(t *testing.T) {
	ci.Parallel(t)

	t.Run("neither set", func(t *testing.T) {
		require.False(t, new(scriptCheckHook).associated("task1", "", ""))
	})

	t.Run("service set", func(t *testing.T) {
		require.True(t, new(scriptCheckHook).associated("task1", "task1", ""))
		require.False(t, new(scriptCheckHook).associated("task1", "task2", ""))
	})

	t.Run("check set", func(t *testing.T) {
		require.True(t, new(scriptCheckHook).associated("task1", "", "task1"))
		require.False(t, new(scriptCheckHook).associated("task1", "", "task2"))
	})

	t.Run("both set", func(t *testing.T) {
		// ensure check.task takes precedence over service.task
		require.True(t, new(scriptCheckHook).associated("task1", "task1", "task1"))
		require.False(t, new(scriptCheckHook).associated("task1", "task1", "task2"))
		require.True(t, new(scriptCheckHook).associated("task1", "task2", "task1"))
		require.False(t, new(scriptCheckHook).associated("task1", "task2", "task2"))
	})
}
