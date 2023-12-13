// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tasklifecycle

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestCoordinator_OnlyMainApp(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	tasks := alloc.Job.TaskGroups[0].Tasks
	task := tasks[0]
	logger := testlog.HCLogger(t)

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)
	coord := NewCoordinator(logger, tasks, shutdownCh)

	// Tasks starts blocked.
	RequireTaskBlocked(t, coord, task)

	// When main is pending it's allowed to run.
	states := map[string]*structs.TaskState{
		task.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskAllowed(t, coord, task)

	// After main is running, main tasks are still allowed to run.
	states = map[string]*structs.TaskState{
		task.Name: {
			State:  structs.TaskStateRunning,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskAllowed(t, coord, task)
}

func TestCoordinator_PrestartRunsBeforeMain(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	sideTask := tasks[1]
	initTask := tasks[2]

	// Only use the tasks that we care about.
	tasks = []*structs.Task{mainTask, sideTask, initTask}

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)
	coord := NewCoordinator(logger, tasks, shutdownCh)

	// All tasks start blocked.
	RequireTaskBlocked(t, coord, initTask)
	RequireTaskBlocked(t, coord, sideTask)
	RequireTaskBlocked(t, coord, mainTask)

	// Set initial state, prestart tasks are allowed to run.
	states := map[string]*structs.TaskState{
		initTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		sideTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskAllowed(t, coord, initTask)
	RequireTaskAllowed(t, coord, sideTask)
	RequireTaskBlocked(t, coord, mainTask)

	// Sidecar task is running, main is blocked.
	states = map[string]*structs.TaskState{
		initTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		sideTask.Name: {
			State:  structs.TaskStateRunning,
			Failed: false,
		},
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskAllowed(t, coord, initTask)
	RequireTaskAllowed(t, coord, sideTask)
	RequireTaskBlocked(t, coord, mainTask)

	// Init task is running, main is blocked.
	states = map[string]*structs.TaskState{
		initTask.Name: {
			State:  structs.TaskStateRunning,
			Failed: false,
		},
		sideTask.Name: {
			State:  structs.TaskStateRunning,
			Failed: false,
		},
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskAllowed(t, coord, initTask)
	RequireTaskAllowed(t, coord, sideTask)
	RequireTaskBlocked(t, coord, mainTask)

	// Init task is done, main is now allowed to run.
	states = map[string]*structs.TaskState{
		initTask.Name: {
			State:  structs.TaskStateDead,
			Failed: false,
		},
		sideTask.Name: {
			State:  structs.TaskStateRunning,
			Failed: false,
		},
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskBlocked(t, coord, initTask)
	RequireTaskAllowed(t, coord, sideTask)
	RequireTaskAllowed(t, coord, mainTask)
}

func TestCoordinator_MainRunsAfterManyInitTasks(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	alloc.Job = mock.VariableLifecycleJob(structs.Resources{CPU: 100, MemoryMB: 256}, 1, 2, 0)
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	init1Task := tasks[1]
	init2Task := tasks[2]

	// Only use the tasks that we care about.
	tasks = []*structs.Task{mainTask, init1Task, init2Task}

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)
	coord := NewCoordinator(logger, tasks, shutdownCh)

	// All tasks start blocked.
	RequireTaskBlocked(t, coord, init1Task)
	RequireTaskBlocked(t, coord, init2Task)
	RequireTaskBlocked(t, coord, mainTask)

	// Set initial state, prestart tasks are allowed to run, main is blocked.
	states := map[string]*structs.TaskState{
		init1Task.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		init2Task.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskAllowed(t, coord, init1Task)
	RequireTaskAllowed(t, coord, init2Task)
	RequireTaskBlocked(t, coord, mainTask)

	// Init tasks complete, main is allowed to run.
	states = map[string]*structs.TaskState{
		init1Task.Name: {
			State:      structs.TaskStateDead,
			Failed:     false,
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		},
		init2Task.Name: {
			State:     structs.TaskStateDead,
			Failed:    false,
			StartedAt: time.Now(),
		},
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskBlocked(t, coord, init1Task)
	RequireTaskBlocked(t, coord, init2Task)
	RequireTaskAllowed(t, coord, mainTask)
}

func TestCoordinator_FailedInitTask(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	alloc.Job = mock.VariableLifecycleJob(structs.Resources{CPU: 100, MemoryMB: 256}, 1, 2, 0)
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	init1Task := tasks[1]
	init2Task := tasks[2]

	// Only use the tasks that we care about.
	tasks = []*structs.Task{mainTask, init1Task, init2Task}

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)
	coord := NewCoordinator(logger, tasks, shutdownCh)

	// All tasks start blocked.
	RequireTaskBlocked(t, coord, init1Task)
	RequireTaskBlocked(t, coord, init2Task)
	RequireTaskBlocked(t, coord, mainTask)

	// Set initial state, prestart tasks are allowed to run, main is blocked.
	states := map[string]*structs.TaskState{
		init1Task.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		init2Task.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskAllowed(t, coord, init1Task)
	RequireTaskAllowed(t, coord, init2Task)
	RequireTaskBlocked(t, coord, mainTask)

	// Init task dies, main is still blocked.
	states = map[string]*structs.TaskState{
		init1Task.Name: {
			State:      structs.TaskStateDead,
			Failed:     false,
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		},
		init2Task.Name: {
			State:     structs.TaskStateDead,
			Failed:    true,
			StartedAt: time.Now(),
		},
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskAllowed(t, coord, init1Task)
	RequireTaskAllowed(t, coord, init2Task)
	RequireTaskBlocked(t, coord, mainTask)
}

func TestCoordinator_SidecarNeverStarts(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	sideTask := tasks[1]
	initTask := tasks[2]

	// Only use the tasks that we care about.
	tasks = []*structs.Task{mainTask, sideTask, initTask}

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)
	coord := NewCoordinator(logger, tasks, shutdownCh)

	// All tasks start blocked.
	RequireTaskBlocked(t, coord, initTask)
	RequireTaskBlocked(t, coord, sideTask)
	RequireTaskBlocked(t, coord, mainTask)

	// Set initial state, prestart tasks are allowed to run, main is blocked.
	states := map[string]*structs.TaskState{
		initTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		sideTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskAllowed(t, coord, initTask)
	RequireTaskAllowed(t, coord, sideTask)
	RequireTaskBlocked(t, coord, mainTask)

	// Init completes, but sidecar not yet.
	states = map[string]*structs.TaskState{
		initTask.Name: {
			State:      structs.TaskStateDead,
			Failed:     false,
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		},
		sideTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskAllowed(t, coord, initTask)
	RequireTaskAllowed(t, coord, sideTask)
	RequireTaskBlocked(t, coord, mainTask)
}

func TestCoordinator_PoststartStartsAfterMain(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	sideTask := tasks[1]
	postTask := tasks[2]

	// Only use the tasks that we care about.
	tasks = []*structs.Task{mainTask, sideTask, postTask}

	// Make the the third task is a poststart hook
	postTask.Lifecycle.Hook = structs.TaskLifecycleHookPoststart

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)
	coord := NewCoordinator(logger, tasks, shutdownCh)

	// All tasks start blocked.
	RequireTaskBlocked(t, coord, sideTask)
	RequireTaskBlocked(t, coord, mainTask)
	RequireTaskBlocked(t, coord, postTask)

	// Set initial state, prestart tasks are allowed to run, main and poststart
	// are blocked.
	states := map[string]*structs.TaskState{
		sideTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		postTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskAllowed(t, coord, sideTask)
	RequireTaskBlocked(t, coord, mainTask)
	RequireTaskBlocked(t, coord, postTask)

	// Sidecar and main running, poststart allowed to run.
	states = map[string]*structs.TaskState{
		sideTask.Name: {
			State:     structs.TaskStateRunning,
			Failed:    false,
			StartedAt: time.Now(),
		},
		mainTask.Name: {
			State:     structs.TaskStateRunning,
			Failed:    false,
			StartedAt: time.Now(),
		},
		postTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.TaskStateUpdated(states)
	RequireTaskAllowed(t, coord, sideTask)
	RequireTaskAllowed(t, coord, mainTask)
	RequireTaskAllowed(t, coord, postTask)
}

func TestCoordinator_Restore(t *testing.T) {
	ci.Parallel(t)

	task := mock.Job().TaskGroups[0].Tasks[0]

	preEphemeral := task.Copy()
	preEphemeral.Name = "pre_ephemeral"
	preEphemeral.Lifecycle = &structs.TaskLifecycleConfig{
		Hook:    structs.TaskLifecycleHookPrestart,
		Sidecar: false,
	}

	preSide := task.Copy()
	preSide.Name = "pre_side"
	preSide.Lifecycle = &structs.TaskLifecycleConfig{
		Hook:    structs.TaskLifecycleHookPrestart,
		Sidecar: true,
	}

	main := task.Copy()
	main.Name = "main"
	main.Lifecycle = nil

	postEphemeral := task.Copy()
	postEphemeral.Name = "post_ephemeral"
	postEphemeral.Lifecycle = &structs.TaskLifecycleConfig{
		Hook:    structs.TaskLifecycleHookPoststart,
		Sidecar: false,
	}

	postSide := task.Copy()
	postSide.Name = "post_side"
	postSide.Lifecycle = &structs.TaskLifecycleConfig{
		Hook:    structs.TaskLifecycleHookPoststart,
		Sidecar: true,
	}

	poststop := task.Copy()
	poststop.Name = "poststop"
	poststop.Lifecycle = &structs.TaskLifecycleConfig{
		Hook:    structs.TaskLifecycleHookPoststop,
		Sidecar: false,
	}

	testCases := []struct {
		name       string
		tasks      []*structs.Task
		tasksState map[string]*structs.TaskState
		testFn     func(*testing.T, *Coordinator)
	}{
		{
			name:  "prestart ephemeral running",
			tasks: []*structs.Task{preEphemeral, preSide, main},
			tasksState: map[string]*structs.TaskState{
				preEphemeral.Name: {State: structs.TaskStateRunning},
				preSide.Name:      {State: structs.TaskStateRunning},
				main.Name:         {State: structs.TaskStatePending},
			},
			testFn: func(t *testing.T, c *Coordinator) {
				RequireTaskBlocked(t, c, main)

				RequireTaskAllowed(t, c, preEphemeral)
				RequireTaskAllowed(t, c, preSide)
			},
		},
		{
			name:  "prestart ephemeral complete",
			tasks: []*structs.Task{preEphemeral, preSide, main},
			tasksState: map[string]*structs.TaskState{
				preEphemeral.Name: {State: structs.TaskStateDead},
				preSide.Name:      {State: structs.TaskStateRunning},
				main.Name:         {State: structs.TaskStatePending},
			},
			testFn: func(t *testing.T, c *Coordinator) {
				RequireTaskBlocked(t, c, preEphemeral)

				RequireTaskAllowed(t, c, preSide)
				RequireTaskAllowed(t, c, main)
			},
		},
		{
			name:  "main running",
			tasks: []*structs.Task{main},
			tasksState: map[string]*structs.TaskState{
				main.Name: {State: structs.TaskStateRunning},
			},
			testFn: func(t *testing.T, c *Coordinator) {
				RequireTaskAllowed(t, c, main)
			},
		},
		{
			name:  "poststart with sidecar",
			tasks: []*structs.Task{main, postEphemeral, postSide},
			tasksState: map[string]*structs.TaskState{
				main.Name:          {State: structs.TaskStateRunning},
				postEphemeral.Name: {State: structs.TaskStateDead},
				postSide.Name:      {State: structs.TaskStateRunning},
			},
			testFn: func(t *testing.T, c *Coordinator) {
				RequireTaskBlocked(t, c, postEphemeral)

				RequireTaskAllowed(t, c, main)
				RequireTaskAllowed(t, c, postSide)
			},
		},
		{
			name:  "poststop running",
			tasks: []*structs.Task{main, poststop},
			tasksState: map[string]*structs.TaskState{
				main.Name:     {State: structs.TaskStateDead},
				poststop.Name: {State: structs.TaskStateRunning},
			},
			testFn: func(t *testing.T, c *Coordinator) {
				RequireTaskBlocked(t, c, main)

				RequireTaskAllowed(t, c, poststop)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shutdownCh := make(chan struct{})
			defer close(shutdownCh)

			c := NewCoordinator(testlog.HCLogger(t), tc.tasks, shutdownCh)
			c.Restore(tc.tasksState)
			tc.testFn(t, c)
		})
	}
}
