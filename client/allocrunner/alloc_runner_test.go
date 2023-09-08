// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allochealth"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	arstate "github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/client/allocrunner/tasklifecycle"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner"
	"github.com/hashicorp/nomad/client/allocwatcher"
	"github.com/hashicorp/nomad/client/lib/proclib"
	"github.com/hashicorp/nomad/client/serviceregistration"
	regMock "github.com/hashicorp/nomad/client/serviceregistration/mock"
	"github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
	"github.com/stretchr/testify/require"
)

// destroy does a blocking destroy on an alloc runner
func destroy(ar interfaces.AllocRunner) {
	ar.Destroy()
	<-ar.DestroyCh()
}

// TestAllocRunner_AllocState_Initialized asserts that getting TaskStates via
// AllocState() are initialized even before the AllocRunner has run.
func TestAllocRunner_AllocState_Initialized(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)

	allocState := ar.AllocState()

	require.NotNil(t, allocState)
	require.NotNil(t, allocState.TaskStates[conf.Alloc.Job.TaskGroups[0].Tasks[0].Name])
}

// TestAllocRunner_TaskLeader_KillTG asserts that when a leader task dies the
// entire task group is killed.
func TestAllocRunner_TaskLeader_KillTG(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 0
	alloc.Job.TaskGroups[0].Tasks[0].RestartPolicy.Attempts = 0

	// Create two tasks in the task group
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "task1"
	task.Driver = "mock_driver"
	task.KillTimeout = 10 * time.Millisecond
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task2 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "task2"
	task2.Driver = "mock_driver"
	task2.Leader = true
	task2.Config = map[string]interface{}{
		"run_for": "1s",
	}
	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2)
	alloc.AllocatedResources.Tasks[task.Name] = tr
	alloc.AllocatedResources.Tasks[task2.Name] = tr

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)
	defer destroy(ar)
	go ar.Run()

	// Wait for all tasks to be killed
	upd := conf.StateUpdater.(*MockStateUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Task1 should be killed because Task2 exited
		state1 := last.TaskStates[task.Name]
		if state1.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state1.State, structs.TaskStateDead)
		}
		if state1.FinishedAt.IsZero() || state1.StartedAt.IsZero() {
			return false, fmt.Errorf("expected to have a start and finish time")
		}
		if len(state1.Events) < 2 {
			// At least have a received and destroyed
			return false, fmt.Errorf("Unexpected number of events")
		}

		found := false
		killingMsg := ""
		for _, e := range state1.Events {
			if e.Type == structs.TaskLeaderDead {
				found = true
			}
			if e.Type == structs.TaskKilling {
				killingMsg = e.DisplayMessage
			}
		}

		if !found {
			return false, fmt.Errorf("Did not find event %v", structs.TaskLeaderDead)
		}

		expectedKillingMsg := "Sent interrupt. Waiting 10ms before force killing"
		if killingMsg != expectedKillingMsg {
			return false, fmt.Errorf("Unexpected task event message - wanted %q. got %q", expectedKillingMsg, killingMsg)
		}

		// Task Two should be dead
		state2 := last.TaskStates[task2.Name]
		if state2.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state2.State, structs.TaskStateDead)
		}
		if state2.FinishedAt.IsZero() || state2.StartedAt.IsZero() {
			return false, fmt.Errorf("expected to have a start and finish time")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// TestAllocRunner_Lifecycle_Poststart asserts that a service job with 2
// poststart lifecycle hooks (1 sidecar, 1 ephemeral) starts all 3 tasks, only
// the ephemeral one finishes, and the other 2 exit when the alloc is stopped.
func TestAllocRunner_Lifecycle_Poststart(t *testing.T) {
	alloc := mock.LifecycleAlloc()

	alloc.Job.Type = structs.JobTypeService
	mainTask := alloc.Job.TaskGroups[0].Tasks[0]
	mainTask.Config["run_for"] = "100s"

	sidecarTask := alloc.Job.TaskGroups[0].Tasks[1]
	sidecarTask.Lifecycle.Hook = structs.TaskLifecycleHookPoststart
	sidecarTask.Config["run_for"] = "100s"

	ephemeralTask := alloc.Job.TaskGroups[0].Tasks[2]
	ephemeralTask.Lifecycle.Hook = structs.TaskLifecycleHookPoststart

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)
	defer destroy(ar)
	go ar.Run()

	upd := conf.StateUpdater.(*MockStateUpdater)

	// Wait for main and sidecar tasks to be running, and that the
	// ephemeral task ran and exited.
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		if last.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("expected alloc to be running not %s", last.ClientStatus)
		}

		if s := last.TaskStates[mainTask.Name].State; s != structs.TaskStateRunning {
			return false, fmt.Errorf("expected main task to be running not %s", s)
		}

		if s := last.TaskStates[sidecarTask.Name].State; s != structs.TaskStateRunning {
			return false, fmt.Errorf("expected sidecar task to be running not %s", s)
		}

		if s := last.TaskStates[ephemeralTask.Name].State; s != structs.TaskStateDead {
			return false, fmt.Errorf("expected ephemeral task to be dead not %s", s)
		}

		if last.TaskStates[ephemeralTask.Name].Failed {
			return false, fmt.Errorf("expected ephemeral task to be successful not failed")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("error waiting for initial state:\n%v", err)
	})

	// Tell the alloc to stop
	stopAlloc := alloc.Copy()
	stopAlloc.DesiredStatus = structs.AllocDesiredStatusStop
	ar.Update(stopAlloc)

	// Wait for main and sidecar tasks to stop.
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()

		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("expected alloc to be running not %s", last.ClientStatus)
		}

		if s := last.TaskStates[mainTask.Name].State; s != structs.TaskStateDead {
			return false, fmt.Errorf("expected main task to be dead not %s", s)
		}

		if last.TaskStates[mainTask.Name].Failed {
			return false, fmt.Errorf("expected main task to be successful not failed")
		}

		if s := last.TaskStates[sidecarTask.Name].State; s != structs.TaskStateDead {
			return false, fmt.Errorf("expected sidecar task to be dead not %s", s)
		}

		if last.TaskStates[sidecarTask.Name].Failed {
			return false, fmt.Errorf("expected sidecar task to be successful not failed")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("error waiting for initial state:\n%v", err)
	})
}

// TestAllocRunner_TaskMain_KillTG asserts that when main tasks die the
// entire task group is killed.
func TestAllocRunner_TaskMain_KillTG(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 0
	alloc.Job.TaskGroups[0].Tasks[0].RestartPolicy.Attempts = 0

	// Create four tasks in the task group
	prestart := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	prestart.Name = "prestart-sidecar"
	prestart.Driver = "mock_driver"
	prestart.KillTimeout = 10 * time.Millisecond
	prestart.Lifecycle = &structs.TaskLifecycleConfig{
		Hook:    structs.TaskLifecycleHookPrestart,
		Sidecar: true,
	}

	prestart.Config = map[string]interface{}{
		"run_for": "100s",
	}

	poststart := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	poststart.Name = "poststart-sidecar"
	poststart.Driver = "mock_driver"
	poststart.KillTimeout = 10 * time.Millisecond
	poststart.Lifecycle = &structs.TaskLifecycleConfig{
		Hook:    structs.TaskLifecycleHookPoststart,
		Sidecar: true,
	}

	poststart.Config = map[string]interface{}{
		"run_for": "100s",
	}

	// these two main tasks have the same name, is that ok?
	main1 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	main1.Name = "task2"
	main1.Driver = "mock_driver"
	main1.Config = map[string]interface{}{
		"run_for": "1s",
	}

	main2 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	main2.Name = "task2"
	main2.Driver = "mock_driver"
	main2.Config = map[string]interface{}{
		"run_for": "2s",
	}

	alloc.Job.TaskGroups[0].Tasks = []*structs.Task{prestart, poststart, main1, main2}
	alloc.AllocatedResources.Tasks = map[string]*structs.AllocatedTaskResources{
		prestart.Name:  tr,
		poststart.Name: tr,
		main1.Name:     tr,
		main2.Name:     tr,
	}

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)
	defer destroy(ar)
	go ar.Run()

	hasTaskMainEvent := func(state *structs.TaskState) bool {
		for _, e := range state.Events {
			if e.Type == structs.TaskMainDead {
				return true
			}
		}

		return false
	}

	// Wait for all tasks to be killed
	upd := conf.StateUpdater.(*MockStateUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		var state *structs.TaskState

		// both sidecars should be killed because Task2 exited
		state = last.TaskStates[prestart.Name]
		if state == nil {
			return false, fmt.Errorf("could not find state for task %s", prestart.Name)
		}
		if state.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state.State, structs.TaskStateDead)
		}
		if state.FinishedAt.IsZero() || state.StartedAt.IsZero() {
			return false, fmt.Errorf("expected to have a start and finish time")
		}
		if len(state.Events) < 2 {
			// At least have a received and destroyed
			return false, fmt.Errorf("Unexpected number of events")
		}

		if !hasTaskMainEvent(state) {
			return false, fmt.Errorf("Did not find event %v: %#+v", structs.TaskMainDead, state.Events)
		}

		state = last.TaskStates[poststart.Name]
		if state == nil {
			return false, fmt.Errorf("could not find state for task %s", poststart.Name)
		}
		if state.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state.State, structs.TaskStateDead)
		}
		if state.FinishedAt.IsZero() || state.StartedAt.IsZero() {
			return false, fmt.Errorf("expected to have a start and finish time")
		}
		if len(state.Events) < 2 {
			// At least have a received and destroyed
			return false, fmt.Errorf("Unexpected number of events")
		}

		if !hasTaskMainEvent(state) {
			return false, fmt.Errorf("Did not find event %v: %#+v", structs.TaskMainDead, state.Events)
		}

		// main tasks should die naturely
		state = last.TaskStates[main1.Name]
		if state.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state.State, structs.TaskStateDead)
		}
		if state.FinishedAt.IsZero() || state.StartedAt.IsZero() {
			return false, fmt.Errorf("expected to have a start and finish time")
		}
		if hasTaskMainEvent(state) {
			return false, fmt.Errorf("unexpected event %#+v in %v", structs.TaskMainDead, state.Events)
		}

		state = last.TaskStates[main2.Name]
		if state.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state.State, structs.TaskStateDead)
		}
		if state.FinishedAt.IsZero() || state.StartedAt.IsZero() {
			return false, fmt.Errorf("expected to have a start and finish time")
		}
		if hasTaskMainEvent(state) {
			return false, fmt.Errorf("unexpected event %v in %#+v", structs.TaskMainDead, state.Events)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// TestAllocRunner_Lifecycle_Poststop asserts that a service job with 1
// postop lifecycle hook starts all 3 tasks, only
// the ephemeral one finishes, and the other 2 exit when the alloc is stopped.
func TestAllocRunner_Lifecycle_Poststop(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.LifecycleAlloc()
	tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]

	alloc.Job.Type = structs.JobTypeService
	mainTask := alloc.Job.TaskGroups[0].Tasks[0]
	mainTask.Config["run_for"] = "100s"

	ephemeralTask := alloc.Job.TaskGroups[0].Tasks[1]
	ephemeralTask.Name = "quit"
	ephemeralTask.Lifecycle.Hook = structs.TaskLifecycleHookPoststop
	ephemeralTask.Config["run_for"] = "10s"

	alloc.Job.TaskGroups[0].Tasks = []*structs.Task{mainTask, ephemeralTask}
	alloc.AllocatedResources.Tasks = map[string]*structs.AllocatedTaskResources{
		mainTask.Name:      tr,
		ephemeralTask.Name: tr,
	}

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)
	defer destroy(ar)
	go ar.Run()

	upd := conf.StateUpdater.(*MockStateUpdater)

	// Wait for main task to be running
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		if last.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("expected alloc to be running not %s", last.ClientStatus)
		}

		if s := last.TaskStates[mainTask.Name].State; s != structs.TaskStateRunning {
			return false, fmt.Errorf("expected main task to be running not %s", s)
		}

		if s := last.TaskStates[ephemeralTask.Name].State; s != structs.TaskStatePending {
			return false, fmt.Errorf("expected ephemeral task to be pending not %s", s)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("error waiting for initial state:\n%v", err)
	})

	// Tell the alloc to stop
	stopAlloc := alloc.Copy()
	stopAlloc.DesiredStatus = structs.AllocDesiredStatusStop
	ar.Update(stopAlloc)

	// Wait for main task to die & poststop task to run.
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()

		if last.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("expected alloc to be running not %s", last.ClientStatus)
		}

		if s := last.TaskStates[mainTask.Name].State; s != structs.TaskStateDead {
			return false, fmt.Errorf("expected main task to be dead not %s", s)
		}

		if s := last.TaskStates[ephemeralTask.Name].State; s != structs.TaskStateRunning {
			return false, fmt.Errorf("expected poststop task to be running not %s", s)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("error waiting for initial state:\n%v", err)
	})

}

func TestAllocRunner_Lifecycle_Restart(t *testing.T) {
	ci.Parallel(t)

	// test cases can use this default or override w/ taskDefs param
	alloc := mock.LifecycleAllocFromTasks([]mock.LifecycleTaskDef{
		{Name: "main", RunFor: "100s", ExitCode: 0, Hook: "", IsSidecar: false},
		{Name: "prestart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "prestart", IsSidecar: false},
		{Name: "prestart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "prestart", IsSidecar: true},
		{Name: "poststart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "poststart", IsSidecar: false},
		{Name: "poststart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "poststart", IsSidecar: true},
		{Name: "poststop", RunFor: "1s", ExitCode: 0, Hook: "poststop", IsSidecar: false},
	})
	alloc.Job.Type = structs.JobTypeService
	rp := &structs.RestartPolicy{
		Attempts: 1,
		Interval: 10 * time.Minute,
		Delay:    1 * time.Nanosecond,
		Mode:     structs.RestartPolicyModeFail,
	}

	ev := &structs.TaskEvent{Type: structs.TaskRestartSignal}

	testCases := []struct {
		name          string
		taskDefs      []mock.LifecycleTaskDef
		isBatch       bool
		hasLeader     bool
		action        func(interfaces.AllocRunner, *structs.Allocation) error
		expectedErr   string
		expectedAfter map[string]structs.TaskState
	}{
		{
			name: "restart entire allocation",
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				return ar.RestartAll(ev)
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "running", Restarts: 1},
				"prestart-oneshot":  {State: "dead", Restarts: 1},
				"prestart-sidecar":  {State: "running", Restarts: 1},
				"poststart-oneshot": {State: "dead", Restarts: 1},
				"poststart-sidecar": {State: "running", Restarts: 1},
				"poststop":          {State: "pending", Restarts: 0},
			},
		},
		{
			name: "restart only running tasks",
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				return ar.RestartRunning(ev)
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "running", Restarts: 1},
				"prestart-oneshot":  {State: "dead", Restarts: 0},
				"prestart-sidecar":  {State: "running", Restarts: 1},
				"poststart-oneshot": {State: "dead", Restarts: 0},
				"poststart-sidecar": {State: "running", Restarts: 1},
				"poststop":          {State: "pending", Restarts: 0},
			},
		},
		{
			name: "batch job restart entire allocation",
			taskDefs: []mock.LifecycleTaskDef{
				{Name: "main", RunFor: "100s", ExitCode: 1, Hook: "", IsSidecar: false},
				{Name: "prestart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "prestart", IsSidecar: false},
				{Name: "prestart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "prestart", IsSidecar: true},
				{Name: "poststart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "poststart", IsSidecar: false},
				{Name: "poststart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "poststart", IsSidecar: true},
				{Name: "poststop", RunFor: "1s", ExitCode: 0, Hook: "poststop", IsSidecar: false},
			},
			isBatch: true,
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				return ar.RestartAll(ev)
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "running", Restarts: 1},
				"prestart-oneshot":  {State: "dead", Restarts: 1},
				"prestart-sidecar":  {State: "running", Restarts: 1},
				"poststart-oneshot": {State: "dead", Restarts: 1},
				"poststart-sidecar": {State: "running", Restarts: 1},
				"poststop":          {State: "pending", Restarts: 0},
			},
		},
		{
			name: "batch job restart only running tasks ",
			taskDefs: []mock.LifecycleTaskDef{
				{Name: "main", RunFor: "100s", ExitCode: 1, Hook: "", IsSidecar: false},
				{Name: "prestart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "prestart", IsSidecar: false},
				{Name: "prestart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "prestart", IsSidecar: true},
				{Name: "poststart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "poststart", IsSidecar: false},
				{Name: "poststart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "poststart", IsSidecar: true},
				{Name: "poststop", RunFor: "1s", ExitCode: 0, Hook: "poststop", IsSidecar: false},
			},
			isBatch: true,
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				return ar.RestartRunning(ev)
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "running", Restarts: 1},
				"prestart-oneshot":  {State: "dead", Restarts: 0},
				"prestart-sidecar":  {State: "running", Restarts: 1},
				"poststart-oneshot": {State: "dead", Restarts: 0},
				"poststart-sidecar": {State: "running", Restarts: 1},
				"poststop":          {State: "pending", Restarts: 0},
			},
		},
		{
			name:      "restart entire allocation with leader",
			hasLeader: true,
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				return ar.RestartAll(ev)
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "running", Restarts: 1},
				"prestart-oneshot":  {State: "dead", Restarts: 1},
				"prestart-sidecar":  {State: "running", Restarts: 1},
				"poststart-oneshot": {State: "dead", Restarts: 1},
				"poststart-sidecar": {State: "running", Restarts: 1},
				"poststop":          {State: "pending", Restarts: 0},
			},
		},
		{
			name: "stop from server",
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				stopAlloc := alloc.Copy()
				stopAlloc.DesiredStatus = structs.AllocDesiredStatusStop
				ar.Update(stopAlloc)
				return nil
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "dead", Restarts: 0},
				"prestart-oneshot":  {State: "dead", Restarts: 0},
				"prestart-sidecar":  {State: "dead", Restarts: 0},
				"poststart-oneshot": {State: "dead", Restarts: 0},
				"poststart-sidecar": {State: "dead", Restarts: 0},
				"poststop":          {State: "dead", Restarts: 0},
			},
		},
		{
			name: "restart main task",
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				return ar.RestartTask("main", ev)
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "running", Restarts: 1},
				"prestart-oneshot":  {State: "dead", Restarts: 0},
				"prestart-sidecar":  {State: "running", Restarts: 0},
				"poststart-oneshot": {State: "dead", Restarts: 0},
				"poststart-sidecar": {State: "running", Restarts: 0},
				"poststop":          {State: "pending", Restarts: 0},
			},
		},
		{
			name:      "restart leader main task",
			hasLeader: true,
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				return ar.RestartTask("main", ev)
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "running", Restarts: 1},
				"prestart-oneshot":  {State: "dead", Restarts: 0},
				"prestart-sidecar":  {State: "running", Restarts: 0},
				"poststart-oneshot": {State: "dead", Restarts: 0},
				"poststart-sidecar": {State: "running", Restarts: 0},
				"poststop":          {State: "pending", Restarts: 0},
			},
		},
		{
			name: "main task fails and restarts once",
			taskDefs: []mock.LifecycleTaskDef{
				{Name: "main", RunFor: "2s", ExitCode: 1, Hook: "", IsSidecar: false},
				{Name: "prestart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "prestart", IsSidecar: false},
				{Name: "prestart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "prestart", IsSidecar: true},
				{Name: "poststart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "poststart", IsSidecar: false},
				{Name: "poststart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "poststart", IsSidecar: true},
				{Name: "poststop", RunFor: "1s", ExitCode: 0, Hook: "poststop", IsSidecar: false},
			},
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				time.Sleep(3 * time.Second) // make sure main task has exited
				return nil
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "dead", Restarts: 1},
				"prestart-oneshot":  {State: "dead", Restarts: 0},
				"prestart-sidecar":  {State: "dead", Restarts: 0},
				"poststart-oneshot": {State: "dead", Restarts: 0},
				"poststart-sidecar": {State: "dead", Restarts: 0},
				"poststop":          {State: "dead", Restarts: 0},
			},
		},
		{
			name: "leader main task fails and restarts once",
			taskDefs: []mock.LifecycleTaskDef{
				{Name: "main", RunFor: "2s", ExitCode: 1, Hook: "", IsSidecar: false},
				{Name: "prestart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "prestart", IsSidecar: false},
				{Name: "prestart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "prestart", IsSidecar: true},
				{Name: "poststart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "poststart", IsSidecar: false},
				{Name: "poststart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "poststart", IsSidecar: true},
				{Name: "poststop", RunFor: "1s", ExitCode: 0, Hook: "poststop", IsSidecar: false},
			},
			hasLeader: true,
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				time.Sleep(3 * time.Second) // make sure main task has exited
				return nil
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "dead", Restarts: 1},
				"prestart-oneshot":  {State: "dead", Restarts: 0},
				"prestart-sidecar":  {State: "dead", Restarts: 0},
				"poststart-oneshot": {State: "dead", Restarts: 0},
				"poststart-sidecar": {State: "dead", Restarts: 0},
				"poststop":          {State: "dead", Restarts: 0},
			},
		},
		{
			name: "main stopped unexpectedly and restarts once",
			taskDefs: []mock.LifecycleTaskDef{
				{Name: "main", RunFor: "2s", ExitCode: 0, Hook: "", IsSidecar: false},
				{Name: "prestart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "prestart", IsSidecar: false},
				{Name: "prestart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "prestart", IsSidecar: true},
				{Name: "poststart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "poststart", IsSidecar: false},
				{Name: "poststart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "poststart", IsSidecar: true},
				{Name: "poststop", RunFor: "1s", ExitCode: 0, Hook: "poststop", IsSidecar: false},
			},
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				time.Sleep(3 * time.Second) // make sure main task has exited
				return nil
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "dead", Restarts: 1},
				"prestart-oneshot":  {State: "dead", Restarts: 0},
				"prestart-sidecar":  {State: "dead", Restarts: 0},
				"poststart-oneshot": {State: "dead", Restarts: 0},
				"poststart-sidecar": {State: "dead", Restarts: 0},
				"poststop":          {State: "dead", Restarts: 0},
			},
		},
		{
			name: "leader main stopped unexpectedly and restarts once",
			taskDefs: []mock.LifecycleTaskDef{
				{Name: "main", RunFor: "2s", ExitCode: 0, Hook: "", IsSidecar: false},
				{Name: "prestart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "prestart", IsSidecar: false},
				{Name: "prestart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "prestart", IsSidecar: true},
				{Name: "poststart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "poststart", IsSidecar: false},
				{Name: "poststart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "poststart", IsSidecar: true},
				{Name: "poststop", RunFor: "1s", ExitCode: 0, Hook: "poststop", IsSidecar: false},
			},
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				time.Sleep(3 * time.Second) // make sure main task has exited
				return nil
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "dead", Restarts: 1},
				"prestart-oneshot":  {State: "dead", Restarts: 0},
				"prestart-sidecar":  {State: "dead", Restarts: 0},
				"poststart-oneshot": {State: "dead", Restarts: 0},
				"poststart-sidecar": {State: "dead", Restarts: 0},
				"poststop":          {State: "dead", Restarts: 0},
			},
		},
		{
			name: "failed main task cannot be restarted",
			taskDefs: []mock.LifecycleTaskDef{
				{Name: "main", RunFor: "2s", ExitCode: 1, Hook: "", IsSidecar: false},
				{Name: "prestart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "prestart", IsSidecar: false},
				{Name: "prestart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "prestart", IsSidecar: true},
				{Name: "poststart-oneshot", RunFor: "1s", ExitCode: 0, Hook: "poststart", IsSidecar: false},
				{Name: "poststart-sidecar", RunFor: "100s", ExitCode: 0, Hook: "poststart", IsSidecar: true},
				{Name: "poststop", RunFor: "1s", ExitCode: 0, Hook: "poststop", IsSidecar: false},
			},
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				// make sure main task has had a chance to restart once on its
				// own and fail again before we try to manually restart it
				time.Sleep(5 * time.Second)
				return ar.RestartTask("main", ev)
			},
			expectedErr: "Task not running",
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "dead", Restarts: 1},
				"prestart-oneshot":  {State: "dead", Restarts: 0},
				"prestart-sidecar":  {State: "dead", Restarts: 0},
				"poststart-oneshot": {State: "dead", Restarts: 0},
				"poststart-sidecar": {State: "dead", Restarts: 0},
				"poststop":          {State: "dead", Restarts: 0},
			},
		},
		{
			name: "restart prestart-sidecar task",
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				return ar.RestartTask("prestart-sidecar", ev)
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "running", Restarts: 0},
				"prestart-oneshot":  {State: "dead", Restarts: 0},
				"prestart-sidecar":  {State: "running", Restarts: 1},
				"poststart-oneshot": {State: "dead", Restarts: 0},
				"poststart-sidecar": {State: "running", Restarts: 0},
				"poststop":          {State: "pending", Restarts: 0},
			},
		},
		{
			name: "restart poststart-sidecar task",
			action: func(ar interfaces.AllocRunner, alloc *structs.Allocation) error {
				return ar.RestartTask("poststart-sidecar", ev)
			},
			expectedAfter: map[string]structs.TaskState{
				"main":              {State: "running", Restarts: 0},
				"prestart-oneshot":  {State: "dead", Restarts: 0},
				"prestart-sidecar":  {State: "running", Restarts: 0},
				"poststart-oneshot": {State: "dead", Restarts: 0},
				"poststart-sidecar": {State: "running", Restarts: 1},
				"poststop":          {State: "pending", Restarts: 0},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)

			alloc := alloc.Copy()
			alloc.Job.TaskGroups[0].RestartPolicy = rp
			if tc.taskDefs != nil {
				alloc = mock.LifecycleAllocFromTasks(tc.taskDefs)
				alloc.Job.Type = structs.JobTypeService
			}
			for _, task := range alloc.Job.TaskGroups[0].Tasks {
				task.RestartPolicy = rp // tasks inherit the group policy
			}
			if tc.hasLeader {
				for _, task := range alloc.Job.TaskGroups[0].Tasks {
					if task.Name == "main" {
						task.Leader = true
					}
				}
			}
			if tc.isBatch {
				alloc.Job.Type = structs.JobTypeBatch
			}

			conf, cleanup := testAllocRunnerConfig(t, alloc)
			defer cleanup()
			ar, err := NewAllocRunner(conf)
			must.NoError(t, err)
			defer destroy(ar)
			go ar.Run()

			upd := conf.StateUpdater.(*MockStateUpdater)

			// assert our "before" states:
			// - all one-shot tasks should be dead but not failed
			// - all main tasks and sidecars should be running
			// - no tasks should have restarted
			testutil.WaitForResult(func() (bool, error) {
				last := upd.Last()
				if last == nil {
					return false, fmt.Errorf("no update")
				}
				if last.ClientStatus != structs.AllocClientStatusRunning {
					return false, fmt.Errorf(
						"expected alloc to be running not %s", last.ClientStatus)
				}
				var errs *multierror.Error

				expectedBefore := map[string]string{
					"main":              "running",
					"prestart-oneshot":  "dead",
					"prestart-sidecar":  "running",
					"poststart-oneshot": "dead",
					"poststart-sidecar": "running",
					"poststop":          "pending",
				}

				for task, expected := range expectedBefore {
					got, ok := last.TaskStates[task]
					if !ok {
						continue
					}
					if got.State != expected {
						errs = multierror.Append(errs, fmt.Errorf(
							"expected initial state of task %q to be %q not %q",
							task, expected, got.State))
					}
					if got.Restarts != 0 {
						errs = multierror.Append(errs, fmt.Errorf(
							"expected no initial restarts of task %q, not %q",
							task, got.Restarts))
					}
					if expected == "dead" && got.Failed {
						errs = multierror.Append(errs, fmt.Errorf(
							"expected ephemeral task %q to be dead but not failed",
							task))
					}

				}
				if errs.ErrorOrNil() != nil {
					return false, errs.ErrorOrNil()
				}
				return true, nil
			}, func(err error) {
				require.NoError(t, err, "error waiting for initial state")
			})

			// perform the action
			err = tc.action(ar, alloc.Copy())
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

			// assert our "after" states
			testutil.WaitForResult(func() (bool, error) {
				last := upd.Last()
				if last == nil {
					return false, fmt.Errorf("no update")
				}
				var errs *multierror.Error
				for task, expected := range tc.expectedAfter {
					got, ok := last.TaskStates[task]
					if !ok {
						errs = multierror.Append(errs, fmt.Errorf(
							"no final state found for task %q", task,
						))
					}
					if got.State != expected.State {
						errs = multierror.Append(errs, fmt.Errorf(
							"expected final state of task %q to be %q not %q",
							task, expected.State, got.State))
					}
					if expected.State == "dead" {
						if got.FinishedAt.IsZero() || got.StartedAt.IsZero() {
							errs = multierror.Append(errs, fmt.Errorf(
								"expected final state of task %q to have start and finish time", task))
						}
						if len(got.Events) < 2 {
							errs = multierror.Append(errs, fmt.Errorf(
								"expected final state of task %q to include at least 2 tasks", task))
						}
					}

					if got.Restarts != expected.Restarts {
						errs = multierror.Append(errs, fmt.Errorf(
							"expected final restarts of task %q to be %v not %v",
							task, expected.Restarts, got.Restarts))
					}
				}
				if errs.ErrorOrNil() != nil {
					return false, errs.ErrorOrNil()
				}
				return true, nil
			}, func(err error) {
				require.NoError(t, err, "error waiting for final state")
			})
		})
	}
}

func TestAllocRunner_TaskGroup_ShutdownDelay(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 0
	alloc.Job.TaskGroups[0].Tasks[0].RestartPolicy.Attempts = 0

	// Create a group service
	tg := alloc.Job.TaskGroups[0]
	tg.Services = []*structs.Service{
		{
			Name:     "shutdown_service",
			Provider: structs.ServiceProviderConsul,
		},
	}

	// Create two tasks in the  group
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "follower1"
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task2 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "leader"
	task2.Driver = "mock_driver"
	task2.Leader = true
	task2.Config = map[string]interface{}{
		"run_for": "10s",
	}

	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2)
	alloc.AllocatedResources.Tasks[task.Name] = tr
	alloc.AllocatedResources.Tasks[task2.Name] = tr

	// Set a shutdown delay
	shutdownDelay := 1 * time.Second
	alloc.Job.TaskGroups[0].ShutdownDelay = &shutdownDelay

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)
	defer destroy(ar)
	go ar.Run()

	// Wait for tasks to start
	upd := conf.StateUpdater.(*MockStateUpdater)
	last := upd.Last()
	testutil.WaitForResult(func() (bool, error) {
		last = upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if n := len(last.TaskStates); n != 2 {
			return false, fmt.Errorf("Not enough task states (want: 2; found %d)", n)
		}
		for name, state := range last.TaskStates {
			if state.State != structs.TaskStateRunning {
				return false, fmt.Errorf("Task %q is not running yet (it's %q)", name, state.State)
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Reset updates
	upd.Reset()

	// Stop alloc
	shutdownInit := time.Now()
	update := alloc.Copy()
	update.DesiredStatus = structs.AllocDesiredStatusStop
	ar.Update(update)

	// Wait for tasks to stop
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		fin := last.TaskStates["leader"].FinishedAt

		if fin.IsZero() {
			return false, nil
		}

		return true, nil
	}, func(err error) {
		last := upd.Last()
		for name, state := range last.TaskStates {
			t.Logf("%s: %s", name, state.State)
		}
		t.Fatalf("err: %v", err)
	})

	// Get consul client operations
	consulClient := conf.Consul.(*regMock.ServiceRegistrationHandler)
	consulOpts := consulClient.GetOps()
	var groupRemoveOp regMock.Operation
	for _, op := range consulOpts {
		// Grab the first deregistration request
		if op.Op == "remove" && op.Name == "group-web" {
			groupRemoveOp = op
			break
		}
	}

	// Ensure remove operation is close to shutdown initiation
	require.True(t, groupRemoveOp.OccurredAt.Sub(shutdownInit) < 100*time.Millisecond)

	last = upd.Last()
	minShutdown := shutdownInit.Add(task.ShutdownDelay)
	leaderFinished := last.TaskStates["leader"].FinishedAt
	followerFinished := last.TaskStates["follower1"].FinishedAt

	// Check that both tasks shut down after min possible shutdown time
	require.Greater(t, leaderFinished.UnixNano(), minShutdown.UnixNano())
	require.Greater(t, followerFinished.UnixNano(), minShutdown.UnixNano())

	// Check that there is at least shutdown_delay between consul
	// remove operation and task finished at time
	require.True(t, leaderFinished.Sub(groupRemoveOp.OccurredAt) > shutdownDelay)
}

// TestAllocRunner_TaskLeader_StopTG asserts that when stopping an alloc with a
// leader the leader is stopped before other tasks.
func TestAllocRunner_TaskLeader_StopTG(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 0
	alloc.Job.TaskGroups[0].Tasks[0].RestartPolicy.Attempts = 0

	// Create 3 tasks in the task group
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "follower1"
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task2 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "leader"
	task2.Driver = "mock_driver"
	task2.Leader = true
	task2.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task3 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task3.Name = "follower2"
	task3.Driver = "mock_driver"
	task3.Config = map[string]interface{}{
		"run_for": "10s",
	}
	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2, task3)
	alloc.AllocatedResources.Tasks[task.Name] = tr
	alloc.AllocatedResources.Tasks[task2.Name] = tr
	alloc.AllocatedResources.Tasks[task3.Name] = tr

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)
	defer destroy(ar)
	go ar.Run()

	// Wait for tasks to start
	upd := conf.StateUpdater.(*MockStateUpdater)
	last := upd.Last()
	testutil.WaitForResult(func() (bool, error) {
		last = upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if n := len(last.TaskStates); n != 3 {
			return false, fmt.Errorf("Not enough task states (want: 3; found %d)", n)
		}
		for name, state := range last.TaskStates {
			if state.State != structs.TaskStateRunning {
				return false, fmt.Errorf("Task %q is not running yet (it's %q)", name, state.State)
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Reset updates
	upd.Reset()

	// Stop alloc
	update := alloc.Copy()
	update.DesiredStatus = structs.AllocDesiredStatusStop
	ar.Update(update)

	// Wait for tasks to stop
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.TaskStates["leader"].FinishedAt.UnixNano() >= last.TaskStates["follower1"].FinishedAt.UnixNano() {
			return false, fmt.Errorf("expected leader to finish before follower1: %s >= %s",
				last.TaskStates["leader"].FinishedAt, last.TaskStates["follower1"].FinishedAt)
		}
		if last.TaskStates["leader"].FinishedAt.UnixNano() >= last.TaskStates["follower2"].FinishedAt.UnixNano() {
			return false, fmt.Errorf("expected leader to finish before follower2: %s >= %s",
				last.TaskStates["leader"].FinishedAt, last.TaskStates["follower2"].FinishedAt)
		}
		return true, nil
	}, func(err error) {
		last := upd.Last()
		for name, state := range last.TaskStates {
			t.Logf("%s: %s", name, state.State)
		}
		t.Fatalf("err: %v", err)
	})
}

// TestAllocRunner_TaskLeader_StopRestoredTG asserts that when stopping a
// restored task group with a leader that failed before restoring the leader is
// not stopped as it does not exist.
// See https://github.com/hashicorp/nomad/issues/3420#issuecomment-341666932
func TestAllocRunner_TaskLeader_StopRestoredTG(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 0
	alloc.Job.TaskGroups[0].Tasks[0].RestartPolicy.Attempts = 0

	// Create a leader and follower task in the task group
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "follower1"
	task.Driver = "mock_driver"
	task.KillTimeout = 10 * time.Second
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task2 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "leader"
	task2.Driver = "mock_driver"
	task2.Leader = true
	task2.KillTimeout = 10 * time.Millisecond
	task2.Config = map[string]interface{}{
		"run_for": "10s",
	}

	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2)
	alloc.AllocatedResources.Tasks[task.Name] = tr
	alloc.AllocatedResources.Tasks[task2.Name] = tr

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	// Use a memory backed statedb
	conf.StateDB = state.NewMemDB(conf.Logger)

	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)

	// setup process wranglers for these tasks
	ar.(*allocRunner).wranglers.Setup(proclib.Task{AllocID: alloc.ID, Task: task.Name})
	ar.(*allocRunner).wranglers.Setup(proclib.Task{AllocID: alloc.ID, Task: task2.Name})

	// Mimic Nomad exiting before the leader stopping is able to stop other tasks.
	ar.(*allocRunner).tasks["leader"].UpdateState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskKilled))
	ar.(*allocRunner).tasks["follower1"].UpdateState(structs.TaskStateRunning, structs.NewTaskEvent(structs.TaskStarted))

	// Create a new AllocRunner to test RestoreState and Run
	ar2, err := NewAllocRunner(conf)
	must.NoError(t, err)
	defer destroy(ar2)

	if err := ar2.Restore(); err != nil {
		t.Fatalf("error restoring state: %v", err)
	}
	ar2.Run()

	// Wait for tasks to be stopped because leader is dead
	testutil.WaitForResult(func() (bool, error) {
		alloc := ar2.Alloc()
		// TODO: this test does not test anything!!! alloc.TaskStates is an empty map
		for task, state := range alloc.TaskStates {
			if state.State != structs.TaskStateDead {
				return false, fmt.Errorf("Task %q should be dead: %v", task, state.State)
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Make sure it GCs properly
	ar2.Destroy()

	select {
	case <-ar2.DestroyCh():
		// exited as expected
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for AR to GC")
	}
}

func TestAllocRunner_Restore_LifecycleHooks(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.LifecycleAlloc()

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	// Use a memory backed statedb
	conf.StateDB = state.NewMemDB(conf.Logger)

	arIface, err := NewAllocRunner(conf)
	must.NoError(t, err)
	ar := arIface.(*allocRunner)

	// Setup the process wranglers for the initial alloc runner, these should
	// be recovered by the second alloc runner.
	ar.wranglers.Setup(proclib.Task{AllocID: alloc.ID, Task: "init"})
	ar.wranglers.Setup(proclib.Task{AllocID: alloc.ID, Task: "side"})
	ar.wranglers.Setup(proclib.Task{AllocID: alloc.ID, Task: "web"})
	ar.wranglers.Setup(proclib.Task{AllocID: alloc.ID, Task: "poststart"})

	go ar.Run()
	defer destroy(ar)

	// Wait for the coordinator to transition from the "init" state.
	tasklifecycle.WaitNotInitUntil(ar.taskCoordinator, time.Second, func() {
		t.Fatalf("task coordinator didn't transition from init in time")
	})

	// We should see all tasks with Prestart hooks are not blocked from running.
	tasklifecycle.RequireTaskAllowed(t, ar.taskCoordinator, ar.tasks["init"].Task())
	tasklifecycle.RequireTaskAllowed(t, ar.taskCoordinator, ar.tasks["side"].Task())
	tasklifecycle.RequireTaskBlocked(t, ar.taskCoordinator, ar.tasks["web"].Task())
	tasklifecycle.RequireTaskBlocked(t, ar.taskCoordinator, ar.tasks["poststart"].Task())

	// Mimic client dies while init task running, and client restarts after
	// init task finished and web is running.
	ar.tasks["init"].UpdateState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskTerminated))
	ar.tasks["side"].UpdateState(structs.TaskStateRunning, structs.NewTaskEvent(structs.TaskStarted))
	ar.tasks["web"].UpdateState(structs.TaskStateRunning, structs.NewTaskEvent(structs.TaskStarted))

	// Create a new AllocRunner to test Restore and Run.
	arIface2, err := NewAllocRunner(conf)
	must.NoError(t, err)
	ar2 := arIface2.(*allocRunner)
	must.NoError(t, ar2.Restore())

	go ar2.Run()
	defer destroy(ar2)

	// Wait for the coordinator to transition from the "init" state.
	tasklifecycle.WaitNotInitUntil(ar.taskCoordinator, time.Second, func() {
		t.Fatalf("task coordinator didn't transition from init in time")
	})

	// Restore resumes execution with correct lifecycle ordering.
	tasklifecycle.RequireTaskBlocked(t, ar2.taskCoordinator, ar2.tasks["init"].Task())
	tasklifecycle.RequireTaskAllowed(t, ar2.taskCoordinator, ar2.tasks["side"].Task())
	tasklifecycle.RequireTaskAllowed(t, ar2.taskCoordinator, ar2.tasks["web"].Task())
	tasklifecycle.RequireTaskAllowed(t, ar2.taskCoordinator, ar2.tasks["poststart"].Task())
}

func TestAllocRunner_Update_Semantics(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	updatedAlloc := func(a *structs.Allocation) *structs.Allocation {
		upd := a.CopySkipJob()
		upd.AllocModifyIndex++

		return upd
	}

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	arIface, err := NewAllocRunner(conf)
	must.NoError(t, err)
	ar := arIface.(*allocRunner)

	upd1 := updatedAlloc(alloc)
	ar.Update(upd1)

	// Update was placed into a queue
	require.Len(ar.allocUpdatedCh, 1)

	upd2 := updatedAlloc(alloc)
	ar.Update(upd2)

	// Allocation was _replaced_

	require.Len(ar.allocUpdatedCh, 1)
	queuedAlloc := <-ar.allocUpdatedCh
	require.Equal(upd2, queuedAlloc)

	// Requeueing older alloc is skipped
	ar.Update(upd2)
	ar.Update(upd1)

	queuedAlloc = <-ar.allocUpdatedCh
	require.Equal(upd2, queuedAlloc)

	// Ignore after watch closed

	close(ar.waitCh)

	ar.Update(upd1)

	// Did not queue the update
	require.Len(ar.allocUpdatedCh, 0)
}

// TestAllocRunner_DeploymentHealth_Healthy_Migration asserts that health is
// reported for services that got migrated; not just part of deployments.
func TestAllocRunner_DeploymentHealth_Healthy_Migration(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()

	// Ensure the alloc is *not* part of a deployment
	alloc.DeploymentID = ""

	// Shorten the default migration healthy time
	tg := alloc.Job.TaskGroups[0]
	tg.Migrate = structs.DefaultMigrateStrategy()
	tg.Migrate.MinHealthyTime = 100 * time.Millisecond
	tg.Migrate.HealthCheck = structs.MigrateStrategyHealthStates

	task := tg.Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "30s",
	}

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)
	go ar.Run()
	defer destroy(ar)

	upd := conf.StateUpdater.(*MockStateUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if !*last.DeploymentStatus.Healthy {
			// This is fatal
			t.Fatal("want deployment status healthy; got unhealthy")
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

// TestAllocRunner_DeploymentHealth_Healthy_NoChecks asserts that the health
// watcher will mark the allocation as healthy based on task states alone.
func TestAllocRunner_DeploymentHealth_Healthy_NoChecks(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()

	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	// Create a task that takes longer to become healthy
	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task.Copy())
	alloc.AllocatedResources.Tasks["task2"] = alloc.AllocatedResources.Tasks["web"].Copy()
	task2 := alloc.Job.TaskGroups[0].Tasks[1]
	task2.Name = "task2"
	task2.Config["start_block_for"] = "500ms"

	// Make the alloc be part of a deployment that uses task states for
	// health checks
	alloc.DeploymentID = uuid.Generate()
	alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_TaskStates
	alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	alloc.Job.TaskGroups[0].Update.MinHealthyTime = 100 * time.Millisecond

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)

	start, done := time.Now(), time.Time{}
	go ar.Run()
	defer destroy(ar)

	upd := conf.StateUpdater.(*MockStateUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if !*last.DeploymentStatus.Healthy {
			// This is fatal
			t.Fatal("want deployment status healthy; got unhealthy")
		}

		// Capture the done timestamp
		done = last.DeploymentStatus.Timestamp
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	if d := done.Sub(start); d < 500*time.Millisecond {
		t.Fatalf("didn't wait for second task group. Only took %v", d)
	}
}

// TestAllocRunner_DeploymentHealth_Unhealthy_Checks asserts that the health
// watcher will mark the allocation as unhealthy with failing checks.
func TestAllocRunner_DeploymentHealth_Unhealthy_Checks(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	// Set a service with check
	task.Services = []*structs.Service{
		{
			Name:      "fakservice",
			PortLabel: "http",
			Checks: []*structs.ServiceCheck{
				{
					Name:     "fakecheck",
					Type:     structs.ServiceCheckScript,
					Command:  "true",
					Interval: 30 * time.Second,
					Timeout:  5 * time.Second,
				},
			},
		},
	}

	// Make the alloc be part of a deployment
	alloc.DeploymentID = uuid.Generate()
	alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_Checks
	alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	alloc.Job.TaskGroups[0].Update.MinHealthyTime = 100 * time.Millisecond
	alloc.Job.TaskGroups[0].Update.HealthyDeadline = 1 * time.Second

	checkUnhealthy := &api.AgentCheck{
		CheckID: uuid.Generate(),
		Status:  api.HealthWarning,
	}

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	// Only return the check as healthy after a duration
	consulClient := conf.Consul.(*regMock.ServiceRegistrationHandler)
	consulClient.AllocRegistrationsFn = func(allocID string) (*serviceregistration.AllocRegistration, error) {
		return &serviceregistration.AllocRegistration{
			Tasks: map[string]*serviceregistration.ServiceRegistrations{
				task.Name: {
					Services: map[string]*serviceregistration.ServiceRegistration{
						"123": {
							Service: &api.AgentService{Service: "fakeservice"},
							Checks:  []*api.AgentCheck{checkUnhealthy},
						},
					},
				},
			},
		}, nil
	}

	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)
	go ar.Run()
	defer destroy(ar)

	var lastUpdate *structs.Allocation
	upd := conf.StateUpdater.(*MockStateUpdater)
	testutil.WaitForResult(func() (bool, error) {
		lastUpdate = upd.Last()
		if lastUpdate == nil {
			return false, fmt.Errorf("No updates")
		}
		if !lastUpdate.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if *lastUpdate.DeploymentStatus.Healthy {
			// This is fatal
			t.Fatal("want deployment status unhealthy; got healthy")
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// Assert that we have an event explaining why we are unhealthy.
	require.Len(t, lastUpdate.TaskStates, 1)
	taskState := lastUpdate.TaskStates[task.Name]
	require.NotNil(t, taskState)
	require.NotEmpty(t, taskState.Events)
	last := taskState.Events[len(taskState.Events)-1]
	require.Equal(t, allochealth.AllocHealthEventSource, last.Type)
	require.Contains(t, last.Message, "by healthy_deadline")
}

// TestAllocRunner_Destroy asserts that Destroy kills and cleans up a running
// alloc.
func TestAllocRunner_Destroy(t *testing.T) {
	ci.Parallel(t)

	// Ensure task takes some time
	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["run_for"] = "10s"

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	// Use a MemDB to assert alloc state gets cleaned up
	conf.StateDB = state.NewMemDB(conf.Logger)

	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)
	go ar.Run()

	// Wait for alloc to be running
	testutil.WaitForResult(func() (bool, error) {
		state := ar.AllocState()

		return state.ClientStatus == structs.AllocClientStatusRunning,
			fmt.Errorf("got client status %v; want running", state.ClientStatus)
	}, func(err error) {
		require.NoError(t, err)
	})

	// Assert state was stored
	ls, ts, err := conf.StateDB.GetTaskRunnerState(alloc.ID, task.Name)
	require.NoError(t, err)
	require.NotNil(t, ls)
	require.NotNil(t, ts)

	// Now destroy
	ar.Destroy()

	select {
	case <-ar.DestroyCh():
		// Destroyed properly!
	case <-time.After(10 * time.Second):
		require.Fail(t, "timed out waiting for alloc to be destroyed")
	}

	// Assert alloc is dead
	state := ar.AllocState()
	require.Equal(t, structs.AllocClientStatusComplete, state.ClientStatus)

	// Assert the state was cleaned
	ls, ts, err = conf.StateDB.GetTaskRunnerState(alloc.ID, task.Name)
	require.NoError(t, err)
	require.Nil(t, ls)
	require.Nil(t, ts)

	// Assert the alloc directory was cleaned
	if _, err := os.Stat(ar.(*allocRunner).allocDir.AllocDir); err == nil {
		require.Fail(t, "alloc dir still exists: %v", ar.(*allocRunner).allocDir.AllocDir)
	} else if !os.IsNotExist(err) {
		require.Failf(t, "expected NotExist error", "found %v", err)
	}
}

func TestAllocRunner_SimpleRun(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)
	go ar.Run()
	defer destroy(ar)

	// Wait for alloc to be running
	testutil.WaitForResult(func() (bool, error) {
		state := ar.AllocState()

		if state.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got status %v; want %v", state.ClientStatus, structs.AllocClientStatusComplete)
		}

		for t, s := range state.TaskStates {
			if s.FinishedAt.IsZero() {
				return false, fmt.Errorf("task %q has zero FinishedAt value", t)
			}
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

}

// TestAllocRunner_MoveAllocDir asserts that a rescheduled
// allocation copies ephemeral disk content from previous alloc run
func TestAllocRunner_MoveAllocDir(t *testing.T) {
	ci.Parallel(t)

	// Step 1: start and run a task
	alloc := mock.BatchAlloc()
	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)

	ar.Run()
	defer destroy(ar)

	WaitForClientState(t, ar, structs.AllocClientStatusComplete)

	// Step 2. Modify its directory
	task := alloc.Job.TaskGroups[0].Tasks[0]
	dataFile := filepath.Join(ar.GetAllocDir().SharedDir, "data", "data_file")
	os.WriteFile(dataFile, []byte("hello world"), os.ModePerm)
	taskDir := ar.GetAllocDir().TaskDirs[task.Name]
	taskLocalFile := filepath.Join(taskDir.LocalDir, "local_file")
	os.WriteFile(taskLocalFile, []byte("good bye world"), os.ModePerm)

	// Step 3. Start a new alloc
	alloc2 := mock.BatchAlloc()
	alloc2.PreviousAllocation = alloc.ID
	alloc2.Job.TaskGroups[0].EphemeralDisk.Sticky = true

	conf2, cleanup := testAllocRunnerConfig(t, alloc2)
	conf2.PrevAllocWatcher, conf2.PrevAllocMigrator = allocwatcher.NewAllocWatcher(allocwatcher.Config{
		Alloc:          alloc2,
		PreviousRunner: ar,
		Logger:         conf2.Logger,
	})
	defer cleanup()
	ar2, err := NewAllocRunner(conf2)
	must.NoError(t, err)

	ar2.Run()
	defer destroy(ar2)

	WaitForClientState(t, ar, structs.AllocClientStatusComplete)

	// Ensure that data from ar was moved to ar2
	dataFile = filepath.Join(ar2.GetAllocDir().SharedDir, "data", "data_file")
	fileInfo, _ := os.Stat(dataFile)
	require.NotNilf(t, fileInfo, "file %q not found", dataFile)

	taskDir = ar2.GetAllocDir().TaskDirs[task.Name]
	taskLocalFile = filepath.Join(taskDir.LocalDir, "local_file")
	fileInfo, _ = os.Stat(taskLocalFile)
	require.NotNilf(t, fileInfo, "file %q not found", dataFile)

}

// TestAllocRuner_HandlesArtifactFailure ensures that if one task in a task group is
// retrying fetching an artifact, other tasks in the group should be able
// to proceed.
func TestAllocRunner_HandlesArtifactFailure(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	rp := &structs.RestartPolicy{
		Mode:     structs.RestartPolicyModeFail,
		Attempts: 1,
		Delay:    time.Nanosecond,
		Interval: time.Hour,
	}
	alloc.Job.TaskGroups[0].RestartPolicy = rp
	alloc.Job.TaskGroups[0].Tasks[0].RestartPolicy = rp

	// Create a new task with a bad artifact
	badtask := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	badtask.Name = "bad"
	badtask.Artifacts = []*structs.TaskArtifact{
		{GetterSource: "http://127.0.0.1:0/foo/bar/baz"},
	}

	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, badtask)
	alloc.AllocatedResources.Tasks["bad"] = &structs.AllocatedTaskResources{
		Cpu: structs.AllocatedCpuResources{
			CpuShares: 500,
		},
		Memory: structs.AllocatedMemoryResources{
			MemoryMB: 256,
		},
	}

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)

	go ar.Run()
	defer destroy(ar)

	testutil.WaitForResult(func() (bool, error) {
		state := ar.AllocState()

		switch state.ClientStatus {
		case structs.AllocClientStatusComplete, structs.AllocClientStatusFailed:
			return true, nil
		default:
			return false, fmt.Errorf("got status %v but want terminal", state.ClientStatus)
		}

	}, func(err error) {
		require.NoError(t, err)
	})

	state := ar.AllocState()
	require.Equal(t, structs.AllocClientStatusFailed, state.ClientStatus)
	require.Equal(t, structs.TaskStateDead, state.TaskStates["web"].State)
	require.True(t, state.TaskStates["web"].Successful())
	require.Equal(t, structs.TaskStateDead, state.TaskStates["bad"].State)
	require.True(t, state.TaskStates["bad"].Failed)
}

// Test that alloc runner kills tasks in task group when another task fails
func TestAllocRunner_TaskFailed_KillTG(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 0
	alloc.Job.TaskGroups[0].Tasks[0].RestartPolicy.Attempts = 0

	// Create two tasks in the task group
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "task1"
	task.Driver = "mock_driver"
	task.KillTimeout = 10 * time.Millisecond
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}
	// Set a service with check
	task.Services = []*structs.Service{
		{
			Name:      "fakservice",
			PortLabel: "http",
			Provider:  structs.ServiceProviderConsul,
			Checks: []*structs.ServiceCheck{
				{
					Name:     "fakecheck",
					Type:     structs.ServiceCheckScript,
					Command:  "true",
					Interval: 30 * time.Second,
					Timeout:  5 * time.Second,
				},
			},
		},
	}

	task2 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "task 2"
	task2.Driver = "mock_driver"
	task2.Config = map[string]interface{}{
		"start_error": "fail task please",
	}
	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2)
	alloc.AllocatedResources.Tasks[task.Name] = tr
	alloc.AllocatedResources.Tasks[task2.Name] = tr

	// Make the alloc be part of a deployment
	alloc.DeploymentID = uuid.Generate()
	alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_Checks
	alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	alloc.Job.TaskGroups[0].Update.MinHealthyTime = 10 * time.Millisecond
	alloc.Job.TaskGroups[0].Update.HealthyDeadline = 2 * time.Second

	checkHealthy := &api.AgentCheck{
		CheckID: uuid.Generate(),
		Status:  api.HealthPassing,
	}

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	consulClient := conf.Consul.(*regMock.ServiceRegistrationHandler)
	consulClient.AllocRegistrationsFn = func(allocID string) (*serviceregistration.AllocRegistration, error) {
		return &serviceregistration.AllocRegistration{
			Tasks: map[string]*serviceregistration.ServiceRegistrations{
				task.Name: {
					Services: map[string]*serviceregistration.ServiceRegistration{
						"123": {
							Service: &api.AgentService{Service: "fakeservice"},
							Checks:  []*api.AgentCheck{checkHealthy},
						},
					},
				},
			},
		}, nil
	}

	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)

	defer destroy(ar)
	go ar.Run()
	upd := conf.StateUpdater.(*MockStateUpdater)

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.ClientStatus != structs.AllocClientStatusFailed {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusFailed)
		}

		// Task One should be killed
		state1 := last.TaskStates[task.Name]
		if state1.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state1.State, structs.TaskStateDead)
		}
		if len(state1.Events) < 2 {
			// At least have a received and destroyed
			return false, fmt.Errorf("Unexpected number of events")
		}

		found := false
		for _, e := range state1.Events {
			if e.Type != structs.TaskSiblingFailed {
				found = true
			}
		}

		if !found {
			return false, fmt.Errorf("Did not find event %v", structs.TaskSiblingFailed)
		}

		// Task Two should be failed
		state2 := last.TaskStates[task2.Name]
		if state2.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state2.State, structs.TaskStateDead)
		}
		if !state2.Failed {
			return false, fmt.Errorf("task2 should have failed")
		}

		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("Expected deployment health to be non nil")
		}

		return true, nil
	}, func(err error) {
		require.Fail(t, "err: %v", err)
	})
}

// Test that alloc becoming terminal should destroy the alloc runner
func TestAllocRunner_TerminalUpdate_Destroy(t *testing.T) {
	ci.Parallel(t)
	alloc := mock.BatchAlloc()
	tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 0
	alloc.Job.TaskGroups[0].Tasks[0].RestartPolicy.Attempts = 0
	// Ensure task takes some time
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["run_for"] = "10s"
	alloc.AllocatedResources.Tasks[task.Name] = tr

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)

	defer destroy(ar)
	go ar.Run()
	upd := conf.StateUpdater.(*MockStateUpdater)

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusRunning)
		}
		return true, nil
	}, func(err error) {
		require.Fail(t, "err: %v", err)
	})

	// Update the alloc to be terminal which should cause the alloc runner to
	// stop the tasks and wait for a destroy.
	update := ar.Alloc().Copy()
	update.DesiredStatus = structs.AllocDesiredStatusStop
	ar.Update(update)

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		// Check the status has changed.
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got client status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Check the alloc directory still exists
		if _, err := os.Stat(ar.GetAllocDir().AllocDir); err != nil {
			return false, fmt.Errorf("alloc dir destroyed: %v", ar.GetAllocDir().AllocDir)
		}

		return true, nil
	}, func(err error) {
		require.Fail(t, "err: %v", err)
	})

	// Send the destroy signal and ensure the AllocRunner cleans up.
	ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		// Check the status has changed.
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got client status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Check the alloc directory was cleaned
		if _, err := os.Stat(ar.GetAllocDir().AllocDir); err == nil {
			return false, fmt.Errorf("alloc dir still exists: %v", ar.GetAllocDir().AllocDir)
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat err: %v", err)
		}

		return true, nil
	}, func(err error) {
		require.Fail(t, "err: %v", err)
	})
}

// TestAllocRunner_PersistState_Destroyed asserts that destroyed allocs don't persist anymore
func TestAllocRunner_PersistState_Destroyed(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	taskName := alloc.Job.LookupTaskGroup(alloc.TaskGroup).Tasks[0].Name

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	conf.StateDB = state.NewMemDB(conf.Logger)

	defer cleanup()
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)

	defer destroy(ar)

	go ar.Run()

	select {
	case <-ar.WaitCh():
	case <-time.After(10 * time.Second):
		require.Fail(t, "timed out waiting for alloc to complete")
	}

	// test final persisted state upon completion
	require.NoError(t, ar.PersistState())
	allocs, _, err := conf.StateDB.GetAllAllocations()
	require.NoError(t, err)
	require.Len(t, allocs, 1)
	require.Equal(t, alloc.ID, allocs[0].ID)
	_, ts, err := conf.StateDB.GetTaskRunnerState(alloc.ID, taskName)
	require.NoError(t, err)
	require.Equal(t, structs.TaskStateDead, ts.State)

	// check that DB alloc is empty after destroying AR
	ar.Destroy()
	select {
	case <-ar.DestroyCh():
	case <-time.After(10 * time.Second):
		require.Fail(t, "timedout waiting for destruction")
	}

	allocs, _, err = conf.StateDB.GetAllAllocations()
	require.NoError(t, err)
	require.Empty(t, allocs)
	_, ts, err = conf.StateDB.GetTaskRunnerState(alloc.ID, taskName)
	require.NoError(t, err)
	require.Nil(t, ts)

	// check that DB alloc is empty after persisting state of destroyed AR
	ar.PersistState()
	allocs, _, err = conf.StateDB.GetAllAllocations()
	require.NoError(t, err)
	require.Empty(t, allocs)
	_, ts, err = conf.StateDB.GetTaskRunnerState(alloc.ID, taskName)
	require.NoError(t, err)
	require.Nil(t, ts)
}

func TestAllocRunner_Reconnect(t *testing.T) {
	ci.Parallel(t)

	type tcase struct {
		clientStatus string
		taskState    string
		taskEvent    *structs.TaskEvent
	}
	tcases := []tcase{
		{
			structs.AllocClientStatusRunning,
			structs.TaskStateRunning,
			structs.NewTaskEvent(structs.TaskStarted),
		},
		{
			structs.AllocClientStatusComplete,
			structs.TaskStateDead,
			structs.NewTaskEvent(structs.TaskTerminated),
		},
		{
			structs.AllocClientStatusFailed,
			structs.TaskStateDead,
			structs.NewTaskEvent(structs.TaskDriverFailure).SetFailsTask(),
		},
		{
			structs.AllocClientStatusPending,
			structs.TaskStatePending,
			structs.NewTaskEvent(structs.TaskReceived),
		},
	}

	for _, tc := range tcases {
		t.Run(tc.clientStatus, func(t *testing.T) {
			// create a running alloc
			alloc := mock.BatchAlloc()
			alloc.AllocModifyIndex = 10
			alloc.ModifyIndex = 10
			alloc.ModifyTime = time.Now().UnixNano()

			// Ensure task takes some time
			task := alloc.Job.TaskGroups[0].Tasks[0]
			task.Driver = "mock_driver"
			task.Config["run_for"] = "30s"

			original := alloc.Copy()

			conf, cleanup := testAllocRunnerConfig(t, alloc)
			defer cleanup()

			ar, err := NewAllocRunner(conf)
			must.NoError(t, err)
			defer destroy(ar)

			go ar.Run()

			for _, taskRunner := range ar.(*allocRunner).tasks {
				taskRunner.UpdateState(tc.taskState, tc.taskEvent)
			}

			update := ar.Alloc().Copy()

			update.ClientStatus = structs.AllocClientStatusUnknown
			update.AllocModifyIndex = original.AllocModifyIndex + 10
			update.ModifyIndex = original.ModifyIndex + 10
			update.ModifyTime = original.ModifyTime + 10

			err = ar.Reconnect(update)
			require.NoError(t, err)

			require.Equal(t, tc.clientStatus, ar.AllocState().ClientStatus)

			// Make sure the runner's alloc indexes match the update.
			require.Equal(t, update.AllocModifyIndex, ar.Alloc().AllocModifyIndex)
			require.Equal(t, update.ModifyIndex, ar.Alloc().ModifyIndex)
			require.Equal(t, update.ModifyTime, ar.Alloc().ModifyTime)

			found := false

			updater := conf.StateUpdater.(*MockStateUpdater)
			var last *structs.Allocation
			testutil.WaitForResult(func() (bool, error) {
				last = updater.Last()
				if last == nil {
					return false, errors.New("last update nil")
				}

				states := last.TaskStates
				for _, s := range states {
					for _, e := range s.Events {
						if e.Type == structs.TaskClientReconnected {
							found = true
							return true, nil
						}
					}
				}

				return false, errors.New("no reconnect event found")
			}, func(err error) {
				require.NoError(t, err)
			})

			require.True(t, found, "no reconnect event found")
		})
	}
}

// TestAllocRunner_Lifecycle_Shutdown_Order asserts that a service job with 3
// lifecycle hooks (1 sidecar, 1 ephemeral, 1 poststop) starts all 4 tasks, and shuts down
// the sidecar after main, but before poststop.
func TestAllocRunner_Lifecycle_Shutdown_Order(t *testing.T) {
	alloc := mock.LifecycleAllocWithPoststopDeploy()

	alloc.Job.Type = structs.JobTypeService

	mainTask := alloc.Job.TaskGroups[0].Tasks[0]
	mainTask.Config["run_for"] = "100s"

	sidecarTask := alloc.Job.TaskGroups[0].Tasks[1]
	sidecarTask.Lifecycle.Hook = structs.TaskLifecycleHookPoststart
	sidecarTask.Config["run_for"] = "100s"

	poststopTask := alloc.Job.TaskGroups[0].Tasks[2]
	ephemeralTask := alloc.Job.TaskGroups[0].Tasks[3]

	alloc.Job.TaskGroups[0].Tasks = []*structs.Task{mainTask, ephemeralTask, sidecarTask, poststopTask}

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	must.NoError(t, err)

	defer destroy(ar)
	go ar.Run()

	upd := conf.StateUpdater.(*MockStateUpdater)

	// Wait for main and sidecar tasks to be running, and that the
	// ephemeral task ran and exited.
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		if last.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("expected alloc to be running not %s", last.ClientStatus)
		}

		if s := last.TaskStates[mainTask.Name].State; s != structs.TaskStateRunning {
			return false, fmt.Errorf("expected main task to be running not %s", s)
		}

		if s := last.TaskStates[sidecarTask.Name].State; s != structs.TaskStateRunning {
			return false, fmt.Errorf("expected sidecar task to be running not %s", s)
		}

		if s := last.TaskStates[ephemeralTask.Name].State; s != structs.TaskStateDead {
			return false, fmt.Errorf("expected ephemeral task to be dead not %s", s)
		}

		if last.TaskStates[ephemeralTask.Name].Failed {
			return false, fmt.Errorf("expected ephemeral task to be successful not failed")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("error waiting for initial state:\n%v", err)
	})

	// Tell the alloc to stop
	stopAlloc := alloc.Copy()
	stopAlloc.DesiredStatus = structs.AllocDesiredStatusStop
	ar.Update(stopAlloc)

	// Wait for tasks to stop.
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()

		if s := last.TaskStates[ephemeralTask.Name].State; s != structs.TaskStateDead {
			return false, fmt.Errorf("expected ephemeral task to be dead not %s", s)
		}

		if last.TaskStates[ephemeralTask.Name].Failed {
			return false, fmt.Errorf("expected ephemeral task to be successful not failed")
		}

		if s := last.TaskStates[mainTask.Name].State; s != structs.TaskStateDead {
			return false, fmt.Errorf("expected main task to be dead not %s", s)
		}

		if last.TaskStates[mainTask.Name].Failed {
			return false, fmt.Errorf("expected main task to be successful not failed")
		}

		if s := last.TaskStates[sidecarTask.Name].State; s != structs.TaskStateDead {
			return false, fmt.Errorf("expected sidecar task to be dead not %s", s)
		}

		if last.TaskStates[sidecarTask.Name].Failed {
			return false, fmt.Errorf("expected sidecar task to be successful not failed")
		}

		if s := last.TaskStates[poststopTask.Name].State; s != structs.TaskStateRunning {
			return false, fmt.Errorf("expected poststop task to be running not %s", s)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("error waiting for kill state:\n%v", err)
	})

	last := upd.Last()
	require.Less(t, last.TaskStates[ephemeralTask.Name].FinishedAt, last.TaskStates[mainTask.Name].FinishedAt)
	require.Less(t, last.TaskStates[mainTask.Name].FinishedAt, last.TaskStates[sidecarTask.Name].FinishedAt)

	// Wait for poststop task to stop.
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()

		if s := last.TaskStates[poststopTask.Name].State; s != structs.TaskStateDead {
			return false, fmt.Errorf("expected poststop task to be dead not %s", s)
		}

		if last.TaskStates[poststopTask.Name].Failed {
			return false, fmt.Errorf("expected poststop task to be successful not failed")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("error waiting for poststop state:\n%v", err)
	})

	last = upd.Last()
	require.Less(t, last.TaskStates[sidecarTask.Name].FinishedAt, last.TaskStates[poststopTask.Name].FinishedAt)
}

func TestHasSidecarTasks(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name           string
		lifecycle      []*structs.TaskLifecycleConfig
		hasSidecars    bool
		hasNonsidecars bool
	}{
		{
			name: "all sidecar - one",
			lifecycle: []*structs.TaskLifecycleConfig{
				{
					Hook:    structs.TaskLifecycleHookPrestart,
					Sidecar: true,
				},
			},
			hasSidecars:    true,
			hasNonsidecars: false,
		},
		{
			name: "all sidecar - multiple",
			lifecycle: []*structs.TaskLifecycleConfig{
				{
					Hook:    structs.TaskLifecycleHookPrestart,
					Sidecar: true,
				},
				{
					Hook:    structs.TaskLifecycleHookPrestart,
					Sidecar: true,
				},
				{
					Hook:    structs.TaskLifecycleHookPrestart,
					Sidecar: true,
				},
			},
			hasSidecars:    true,
			hasNonsidecars: false,
		},
		{
			name: "some sidecars, some others",
			lifecycle: []*structs.TaskLifecycleConfig{
				nil,
				{
					Hook:    structs.TaskLifecycleHookPrestart,
					Sidecar: false,
				},
				{
					Hook:    structs.TaskLifecycleHookPrestart,
					Sidecar: true,
				},
			},
			hasSidecars:    true,
			hasNonsidecars: true,
		},
		{
			name: "no sidecars",
			lifecycle: []*structs.TaskLifecycleConfig{
				nil,
				{
					Hook:    structs.TaskLifecycleHookPrestart,
					Sidecar: false,
				},
				nil,
			},
			hasSidecars:    false,
			hasNonsidecars: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create alloc with the given task lifecycle configurations.
			alloc := mock.BatchAlloc()

			tasks := []*structs.Task{}
			resources := map[string]*structs.AllocatedTaskResources{}

			tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]

			for i, lifecycle := range tc.lifecycle {
				task := alloc.Job.TaskGroups[0].Tasks[0].Copy()
				task.Name = fmt.Sprintf("task%d", i)
				task.Lifecycle = lifecycle
				tasks = append(tasks, task)
				resources[task.Name] = tr
			}

			alloc.Job.TaskGroups[0].Tasks = tasks
			alloc.AllocatedResources.Tasks = resources

			// Create alloc runner.
			arConf, cleanup := testAllocRunnerConfig(t, alloc)
			defer cleanup()

			arIface, err := NewAllocRunner(arConf)
			must.NoError(t, err)
			ar := arIface.(*allocRunner)

			require.Equal(t, tc.hasSidecars, hasSidecarTasks(ar.tasks), "sidecars")

			runners := []*taskrunner.TaskRunner{}
			for _, r := range ar.tasks {
				runners = append(runners, r)
			}
			require.Equal(t, tc.hasNonsidecars, hasNonSidecarTasks(runners), "non-sidecars")

		})
	}
}

type allocPreKillHook struct {
	ran atomic.Bool
}

func (*allocPreKillHook) Name() string { return "test_prekill" }

func (h *allocPreKillHook) PreKill() {
	h.ran.Store(true)
}

func TestAllocRunner_PreKill_RunOnDone(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{"run_for": "2ms"}
	alloc.DesiredStatus = "stop"

	conf, cleanup := testAllocRunnerConfig(t, alloc.Copy())
	t.Cleanup(cleanup)

	arIface, err := NewAllocRunner(conf)
	must.NoError(t, err)
	ar := arIface.(*allocRunner)

	// set our custom prekill hook
	hook := new(allocPreKillHook)
	ar.runnerHooks = append(ar.runnerHooks, hook)

	go ar.Run()
	defer destroy(ar)

	// wait for completion or timeout
	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(hook.ran.Load),
		wait.Timeout(5*time.Second),
		wait.Gap(500*time.Millisecond),
	))
}

func TestAllocRunner_GetUpdatePriority(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{"run_for": "2ms"}
	alloc.DesiredStatus = "stop"

	conf, cleanup := testAllocRunnerConfig(t, alloc.Copy())
	t.Cleanup(cleanup)

	arIface, err := NewAllocRunner(conf)
	must.NoError(t, err)
	ar := arIface.(*allocRunner)

	ar.SetNetworkStatus(&structs.AllocNetworkStatus{
		InterfaceName: "eth0",
		Address:       "192.168.1.1",
		DNS:           &structs.DNSConfig{},
	})

	calloc := ar.clientAlloc(map[string]*structs.TaskState{})
	ar.AcknowledgeState(&arstate.State{
		ClientStatus:      calloc.ClientStatus,
		ClientDescription: calloc.ClientDescription,
		DeploymentStatus:  calloc.DeploymentStatus,
		TaskStates:        calloc.TaskStates,
		NetworkStatus:     calloc.NetworkStatus,
	})

	must.Eq(t, cstructs.AllocUpdatePriorityNone, ar.GetUpdatePriority(calloc))

	// clientAlloc mutates the state, so verify this doesn't break the check
	// without state having been updated
	calloc = ar.clientAlloc(map[string]*structs.TaskState{})
	must.Eq(t, cstructs.AllocUpdatePriorityNone, ar.GetUpdatePriority(calloc))

	// make a no-op state update
	ar.SetNetworkStatus(&structs.AllocNetworkStatus{
		InterfaceName: "eth0",
		Address:       "192.168.1.1",
		DNS:           &structs.DNSConfig{},
	})
	calloc = ar.clientAlloc(map[string]*structs.TaskState{})
	must.Eq(t, cstructs.AllocUpdatePriorityNone, ar.GetUpdatePriority(calloc))

	// make a low priority state update
	ar.SetNetworkStatus(&structs.AllocNetworkStatus{
		InterfaceName: "eth0",
		Address:       "192.168.1.2",
		DNS:           &structs.DNSConfig{},
	})
	calloc = ar.clientAlloc(map[string]*structs.TaskState{})
	must.Eq(t, cstructs.AllocUpdatePriorityTypical, ar.GetUpdatePriority(calloc))

	// make a state update that should be detected as high priority
	ar.SetClientStatus(structs.AllocClientStatusFailed)
	calloc = ar.clientAlloc(map[string]*structs.TaskState{})
	must.Eq(t, cstructs.AllocUpdatePriorityUrgent, ar.GetUpdatePriority(calloc))
}
