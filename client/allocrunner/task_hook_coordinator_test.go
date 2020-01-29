package allocrunner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
)

func TestTaskHookCoordinator_OnlyMainApp(t *testing.T) {
	alloc := mock.Alloc()
	tasks := alloc.Job.TaskGroups[0].Tasks
	task := tasks[0]
	logger := testlog.HCLogger(t)

	coord := newTaskHookCoordinator(logger, tasks)

	ch := coord.startConditionForTask(task)

	require.Truef(t, isChannelClosed(ch), "%s channel was open, should be closed", task.Name)
}

func TestTaskHookCoordinator_PrestartRunsBeforeMain(t *testing.T) {
	logger := testlog.HCLogger(t)

	alloc := mock.Alloc()
	tasks := alloc.Job.TaskGroups[0].Tasks
	tasks = append(tasks, initTask())
	tasks = append(tasks, sidecarTask())

	mainTask := tasks[0]
	initTask := tasks[1]
	sideTask := tasks[2]

	coord := newTaskHookCoordinator(logger, tasks)
	mainCh := coord.startConditionForTask(mainTask)
	initCh := coord.startConditionForTask(initTask)
	sideCh := coord.startConditionForTask(sideTask)

	require.Falsef(t, isChannelClosed(mainCh), "%s channel was closed, should be open", mainTask.Name)
	require.Truef(t, isChannelClosed(initCh), "%s channel was open, should be closed", initTask.Name)
	require.Truef(t, isChannelClosed(sideCh), "%s channel was open, should be closed", sideTask.Name)
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

	require.Falsef(t, isChannelClosed(mainCh), "%s channel was closed, should be open", mainTaskName)
	require.Truef(t, isChannelClosed(initCh), "%s channel was open, should be closed", initTaskName)
	require.Truef(t, isChannelClosed(sideCh), "%s channel was open, should be closed", sideTaskName)

	states := map[string]*structs.TaskState{
		mainTaskName: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		initTaskName: {
			State:      structs.TaskStateDead,
			Failed:     false,
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		},
		sideTaskName: {
			State:     structs.TaskStateRunning,
			Failed:    false,
			StartedAt: time.Now(),
		},
	}

	coord.taskStateUpdated(states)

	require.Truef(t, isChannelClosed(initCh), "%s channel was open, should be closed", mainTaskName)
	require.Truef(t, isChannelClosed(initCh), "%s channel was open, should be closed", initTaskName)
	require.Truef(t, isChannelClosed(sideCh), "%s channel was open, should be closed", sideTaskName)
}

func TestTaskHookCoordinator_MainRunsAfterManyInitTasks(t *testing.T) {
	logger := testlog.HCLogger(t)

	alloc := mock.Alloc()
	tasks := alloc.Job.TaskGroups[0].Tasks
	init1 := initTask()
	init1.Name = "init-1"
	init2 := initTask()
	init2.Name = "init-2"

	tasks = append(tasks, init1)
	tasks = append(tasks, init2)

	coord := newTaskHookCoordinator(logger, tasks)
	mainCh := coord.startConditionForTask(tasks[0])
	init1Ch := coord.startConditionForTask(tasks[1])
	init2Ch := coord.startConditionForTask(tasks[2])

	require.Falsef(t, isChannelClosed(mainCh), "%s channel was closed, should be open", tasks[0].Name)
	require.Truef(t, isChannelClosed(init1Ch), "%s channel was open, should be closed", tasks[1].Name)
	require.Truef(t, isChannelClosed(init2Ch), "%s channel was open, should be closed", tasks[2].Name)

	states := map[string]*structs.TaskState{
		tasks[0].Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		tasks[1].Name: {
			State:      structs.TaskStateDead,
			Failed:     false,
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		},
		tasks[2].Name: {
			State:     structs.TaskStateDead,
			Failed:    false,
			StartedAt: time.Now(),
		},
	}

	coord.taskStateUpdated(states)

	require.Truef(t, isChannelClosed(init1Ch), "%s channel was open, should be closed", tasks[0].Name)
	require.Truef(t, isChannelClosed(init2Ch), "%s channel was open, should be closed", tasks[1].Name)
	require.Truef(t, isChannelClosed(mainCh), "%s channel was open, should be closed", tasks[2].Name)
}

func isChannelClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
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
