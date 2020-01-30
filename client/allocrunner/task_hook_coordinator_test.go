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

	alloc := mock.LifecycleAlloc()
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	sideTask := tasks[1]
	initTask := tasks[2]

	coord := newTaskHookCoordinator(logger, tasks)
	initCh := coord.startConditionForTask(initTask)
	sideCh := coord.startConditionForTask(sideTask)
	mainCh := coord.startConditionForTask(mainTask)

	require.Truef(t, isChannelClosed(initCh), "%s channel was open, should be closed", initTask.Name)
	require.Truef(t, isChannelClosed(sideCh), "%s channel was open, should be closed", sideTask.Name)
	require.Falsef(t, isChannelClosed(mainCh), "%s channel was closed, should be open", mainTask.Name)
}

func TestTaskHookCoordinator_MainRunsAfterPrestart(t *testing.T) {
	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	sideTask := tasks[1]
	initTask := tasks[2]

	coord := newTaskHookCoordinator(logger, tasks)
	initCh := coord.startConditionForTask(initTask)
	sideCh := coord.startConditionForTask(sideTask)
	mainCh := coord.startConditionForTask(mainTask)

	require.Truef(t, isChannelClosed(initCh), "%s channel was open, should be closed", initTask.Name)
	require.Truef(t, isChannelClosed(sideCh), "%s channel was open, should be closed", sideTask.Name)
	require.Falsef(t, isChannelClosed(mainCh), "%s channel was closed, should be open", mainTask.Name)

	states := map[string]*structs.TaskState{
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		initTask.Name: {
			State:      structs.TaskStateDead,
			Failed:     false,
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		},
		sideTask.Name: {
			State:     structs.TaskStateRunning,
			Failed:    false,
			StartedAt: time.Now(),
		},
	}

	coord.taskStateUpdated(states)

	require.Truef(t, isChannelClosed(initCh), "%s channel was open, should be closed", initTask.Name)
	require.Truef(t, isChannelClosed(sideCh), "%s channel was open, should be closed", sideTask.Name)
	require.Truef(t, isChannelClosed(mainCh), "%s channel was open, should be closed", mainTask.Name)
}

func TestTaskHookCoordinator_MainRunsAfterManyInitTasks(t *testing.T) {
	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	alloc.Job = mock.VariableLifecycleJob(structs.Resources{CPU: 100, MemoryMB: 256}, 1, 2, 0)
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	init1Task := tasks[1]
	init2Task := tasks[2]

	coord := newTaskHookCoordinator(logger, tasks)
	mainCh := coord.startConditionForTask(mainTask)
	init1Ch := coord.startConditionForTask(init1Task)
	init2Ch := coord.startConditionForTask(init2Task)

	require.Truef(t, isChannelClosed(init1Ch), "%s channel was open, should be closed", init1Task.Name)
	require.Truef(t, isChannelClosed(init2Ch), "%s channel was open, should be closed", init2Task.Name)
	require.Falsef(t, isChannelClosed(mainCh), "%s channel was closed, should be open", mainTask.Name)

	states := map[string]*structs.TaskState{
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
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
	}

	coord.taskStateUpdated(states)

	require.Truef(t, isChannelClosed(init1Ch), "%s channel was open, should be closed", init1Task.Name)
	require.Truef(t, isChannelClosed(init2Ch), "%s channel was open, should be closed", init2Task.Name)
	require.Truef(t, isChannelClosed(mainCh), "%s channel was open, should be closed", mainTask.Name)
}

func TestTaskHookCoordinator_FailedInitTask(t *testing.T) {
	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	alloc.Job = mock.VariableLifecycleJob(structs.Resources{CPU: 100, MemoryMB: 256}, 1, 2, 0)
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	init1Task := tasks[1]
	init2Task := tasks[2]

	coord := newTaskHookCoordinator(logger, tasks)
	mainCh := coord.startConditionForTask(mainTask)
	init1Ch := coord.startConditionForTask(init1Task)
	init2Ch := coord.startConditionForTask(init2Task)

	require.Truef(t, isChannelClosed(init1Ch), "%s channel was open, should be closed", init1Task.Name)
	require.Truef(t, isChannelClosed(init2Ch), "%s channel was open, should be closed", init2Task.Name)
	require.Falsef(t, isChannelClosed(mainCh), "%s channel was closed, should be open", mainTask.Name)

	states := map[string]*structs.TaskState{
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
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
	}

	coord.taskStateUpdated(states)

	require.Truef(t, isChannelClosed(init1Ch), "%s channel was open, should be closed", init1Task.Name)
	require.Truef(t, isChannelClosed(init2Ch), "%s channel was open, should be closed", init2Task.Name)
	require.Falsef(t, isChannelClosed(mainCh), "%s channel was closed, should be open", mainTask.Name)
}

func TestTaskHookCoordinator_SidecarNeverStarts(t *testing.T) {
	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	sideTask := tasks[1]
	initTask := tasks[2]

	coord := newTaskHookCoordinator(logger, tasks)
	initCh := coord.startConditionForTask(initTask)
	sideCh := coord.startConditionForTask(sideTask)
	mainCh := coord.startConditionForTask(mainTask)

	require.Truef(t, isChannelClosed(initCh), "%s channel was open, should be closed", initTask.Name)
	require.Truef(t, isChannelClosed(sideCh), "%s channel was open, should be closed", sideTask.Name)
	require.Falsef(t, isChannelClosed(mainCh), "%s channel was closed, should be open", mainTask.Name)

	states := map[string]*structs.TaskState{
		mainTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
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
	}

	coord.taskStateUpdated(states)

	require.Truef(t, isChannelClosed(initCh), "%s channel was open, should be closed", initTask.Name)
	require.Truef(t, isChannelClosed(sideCh), "%s channel was open, should be closed", sideTask.Name)
	require.Falsef(t, isChannelClosed(mainCh), "%s channel was closed, should be open", mainTask.Name)
}

func isChannelClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
