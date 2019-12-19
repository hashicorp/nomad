package allocrunner

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/stretchr/testify/require"
)

func TestTaskHookCoordinator_OnlyMainApp(t *testing.T) {
	alloc := mock.Alloc()
	tasks := alloc.Job.TaskGroups[0].Tasks
	logger := testlog.HCLogger(t)

	coord := newTaskHookCoordinator(logger, tasks)

	ch := coord.startConditionForTask(tasks[0])

	testChannelClosed(t, ch, tasks[0].Name)
}

func TestTaskHookCoordinator_PrestartRunsBeforeMain(t *testing.T) {
	logger := testlog.HCLogger(t)

	alloc := mock.Alloc()
	tasks := alloc.Job.TaskGroups[0].Tasks
	tasks = append(tasks, initTask())
	tasks = append(tasks, sidecarTask())

	coord := newTaskHookCoordinator(logger, tasks)
	mainCh := coord.startConditionForTask(tasks[0])
	initCh := coord.startConditionForTask(tasks[1])
	sideCh := coord.startConditionForTask(tasks[2])

	mainTaskName := tasks[0].Name
	initTaskName := tasks[1].Name
	sideTaskName := tasks[2].Name

	testChannelOpen(t, mainCh, mainTaskName)
	testChannelClosed(t, initCh, initTaskName)
	testChannelClosed(t, sideCh, sideTaskName)
}

func TestTaskHookCoordinator_MainRunsAfterPrestart(t *testing.T) {
	logger := testlog.HCLogger(t)

	alloc := mock.Alloc()
	tasks := alloc.Job.TaskGroups[0].Tasks
	tasks = append(tasks, initTask())
	tasks = append(tasks, sidecarTask())

	coord := newTaskHookCoordinator(logger, tasks)
	mainCh := coord.startConditionForTask(tasks[0])
	initCh := coord.startConditionForTask(tasks[1])
	sideCh := coord.startConditionForTask(tasks[2])

	mainTaskName := tasks[0].Name
	initTaskName := tasks[1].Name
	sideTaskName := tasks[2].Name

	testChannelOpen(t, mainCh, mainTaskName)
	testChannelClosed(t, initCh, initTaskName)
	testChannelClosed(t, sideCh, sideTaskName)

	states := map[string]*structs.TaskState{
		mainTaskName: &structs.TaskState{
			State:  structs.TaskStatePending,
			Failed: false,
		},
		initTaskName: &structs.TaskState{
			State:      structs.TaskStateDead,
			Failed:     false,
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		},
		sideTaskName: &structs.TaskState{
			State:     structs.TaskStateRunning,
			Failed:    false,
			StartedAt: time.Now(),
		},
	}

	coord.taskStateUpdated(states)

	testChannelClosed(t, mainCh, mainTaskName)
	testChannelClosed(t, initCh, initTaskName)
	testChannelClosed(t, sideCh, sideTaskName)
}

func testChannelOpen(t *testing.T, ch <-chan struct{}, name string) {
	select {
	case <-ch:
		require.Failf(t, "channel was closed, should be open", name)
	default:
		// channel is open
	}
}

func testChannelClosed(t *testing.T, ch <-chan struct{}, name string) {
	select {
	case _, ok := <-ch:
		require.False(t, ok)
	default:
		// channel is open
		require.Failf(t, "channel was open, should be closed", name)
	}
}

func sidecarTask() *structs.Task {
	return &structs.Task{
		Name: "sidecar",
		Lifecycle: &structs.TaskLifecycleConfig{
			Hook:       structs.TaskLifecycleHookPrestart,
			BlockUntil: structs.TaskLifecycleBlockUntilRunning,
		},
	}
}

func initTask() *structs.Task {
	return &structs.Task{
		Name: "init",
		Lifecycle: &structs.TaskLifecycleConfig{
			Hook:       structs.TaskLifecycleHookPrestart,
			BlockUntil: structs.TaskLifecycleBlockUntilCompleted,
		},
	}
}
