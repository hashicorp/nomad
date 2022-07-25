package allocrunner

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestTaskCoordinator_OnlyMainApp(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	tasks := alloc.Job.TaskGroups[0].Tasks
	task := tasks[0]
	logger := testlog.HCLogger(t)

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)
	coord := newTaskCoordinator(logger, tasks, shutdownCh)

	// Tasks starts blocked.
	requireTaskBlocked(t, coord, task)

	// When main is pending it's allowed to run.
	states := map[string]*structs.TaskState{
		task.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
	}
	coord.taskStateUpdated(states)
	requireTaskAllowed(t, coord, task)

	// After main is running, main tasks are still allowed to run.
	states = map[string]*structs.TaskState{
		task.Name: {
			State:  structs.TaskStateRunning,
			Failed: false,
		},
	}
	coord.taskStateUpdated(states)
	requireTaskAllowed(t, coord, task)
}

func TestTaskCoordinator_PrestartRunsBeforeMain(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	sideTask := tasks[1]
	initTask := tasks[2]

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)
	coord := newTaskCoordinator(logger, tasks, shutdownCh)

	// All tasks start blocked.
	requireTaskBlocked(t, coord, initTask)
	requireTaskBlocked(t, coord, sideTask)
	requireTaskBlocked(t, coord, mainTask)

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
	coord.taskStateUpdated(states)
	requireTaskAllowed(t, coord, initTask)
	requireTaskAllowed(t, coord, sideTask)
	requireTaskBlocked(t, coord, mainTask)

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
	coord.taskStateUpdated(states)
	requireTaskAllowed(t, coord, initTask)
	requireTaskAllowed(t, coord, sideTask)
	requireTaskBlocked(t, coord, mainTask)

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
	coord.taskStateUpdated(states)
	requireTaskAllowed(t, coord, initTask)
	requireTaskAllowed(t, coord, sideTask)
	requireTaskBlocked(t, coord, mainTask)

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
	coord.taskStateUpdated(states)
	requireTaskBlocked(t, coord, initTask)
	requireTaskAllowed(t, coord, sideTask)
	requireTaskAllowed(t, coord, mainTask)
}

func TestTaskCoordinator_MainRunsAfterManyInitTasks(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	alloc.Job = mock.VariableLifecycleJob(structs.Resources{CPU: 100, MemoryMB: 256}, 1, 2, 0)
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	init1Task := tasks[1]
	init2Task := tasks[2]

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)
	coord := newTaskCoordinator(logger, tasks, shutdownCh)

	// All tasks start blocked.
	requireTaskBlocked(t, coord, init1Task)
	requireTaskBlocked(t, coord, init2Task)
	requireTaskBlocked(t, coord, mainTask)

	// Set initial state, prestart tasks are allowed to run, mais is blocked.
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
	coord.taskStateUpdated(states)
	requireTaskAllowed(t, coord, init1Task)
	requireTaskAllowed(t, coord, init2Task)
	requireTaskBlocked(t, coord, mainTask)

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
	coord.taskStateUpdated(states)
	requireTaskBlocked(t, coord, init1Task)
	requireTaskBlocked(t, coord, init2Task)
	requireTaskAllowed(t, coord, mainTask)
}

func TestTaskCoordinator_FailedInitTask(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	alloc.Job = mock.VariableLifecycleJob(structs.Resources{CPU: 100, MemoryMB: 256}, 1, 2, 0)
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	init1Task := tasks[1]
	init2Task := tasks[2]

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)
	coord := newTaskCoordinator(logger, tasks, shutdownCh)

	// All tasks start blocked.
	requireTaskBlocked(t, coord, init1Task)
	requireTaskBlocked(t, coord, init2Task)
	requireTaskBlocked(t, coord, mainTask)

	// Set initial state, prestart tasks are allowed to run, mais is blocked.
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
	coord.taskStateUpdated(states)
	requireTaskAllowed(t, coord, init1Task)
	requireTaskAllowed(t, coord, init2Task)
	requireTaskBlocked(t, coord, mainTask)

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
	coord.taskStateUpdated(states)
	requireTaskAllowed(t, coord, init1Task)
	requireTaskAllowed(t, coord, init2Task)
	requireTaskBlocked(t, coord, mainTask)
}

func TestTaskCoordinator_SidecarNeverStarts(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	sideTask := tasks[1]
	initTask := tasks[2]

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)
	coord := newTaskCoordinator(logger, tasks, shutdownCh)

	// All tasks start blocked.
	requireTaskBlocked(t, coord, initTask)
	requireTaskBlocked(t, coord, sideTask)
	requireTaskBlocked(t, coord, mainTask)

	// Set initial state, prestart tasks are allowed to run, mais is blocked.
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
	coord.taskStateUpdated(states)
	requireTaskAllowed(t, coord, initTask)
	requireTaskAllowed(t, coord, sideTask)
	requireTaskBlocked(t, coord, mainTask)

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
	coord.taskStateUpdated(states)
	requireTaskAllowed(t, coord, initTask)
	requireTaskAllowed(t, coord, sideTask)
	requireTaskBlocked(t, coord, mainTask)
}

func TestTaskCoordinator_PoststartStartsAfterMain(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	sideTask := tasks[1]
	postTask := tasks[2]

	// Make the the third task is a poststart hook
	postTask.Lifecycle.Hook = structs.TaskLifecycleHookPoststart

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)
	coord := newTaskCoordinator(logger, tasks, shutdownCh)

	// All tasks start blocked.
	requireTaskBlocked(t, coord, sideTask)
	requireTaskBlocked(t, coord, mainTask)
	requireTaskBlocked(t, coord, postTask)

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
	coord.taskStateUpdated(states)
	requireTaskAllowed(t, coord, sideTask)
	requireTaskBlocked(t, coord, mainTask)
	requireTaskBlocked(t, coord, postTask)

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
	coord.taskStateUpdated(states)
	requireTaskAllowed(t, coord, sideTask)
	requireTaskAllowed(t, coord, mainTask)
	requireTaskAllowed(t, coord, postTask)
}

func requireTaskBlocked(t *testing.T, c *taskCoordinator, task *structs.Task) {
	ch := c.startConditionForTask(task)
	requireChannelBlocking(t, ch, task.Name)
}

func requireTaskAllowed(t *testing.T, c *taskCoordinator, task *structs.Task) {
	ch := c.startConditionForTask(task)
	requireChannelPassing(t, ch, task.Name)
}

func requireChannelPassing(t *testing.T, ch <-chan struct{}, name string) {
	testutil.WaitForResult(func() (bool, error) {
		return !isChannelBlocking(ch), nil
	}, func(_ error) {
		t.Fatalf("%s channel was blocking, should be passing", name)
	})
}

func requireChannelBlocking(t *testing.T, ch <-chan struct{}, name string) {
	testutil.WaitForResult(func() (bool, error) {
		return isChannelBlocking(ch), nil
	}, func(_ error) {
		t.Fatalf("%s channel was passing, should be blocking", name)
	})
}

func isChannelBlocking(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return false
	default:
		return true
	}
}
