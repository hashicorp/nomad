package allocrunner

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/client/allocrunner/taskrunner"
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

func TestTaskHookCoordinator_PoststartStartsAfterMain(t *testing.T) {
	logger := testlog.HCLogger(t)

	alloc := mock.LifecycleAlloc()
	tasks := alloc.Job.TaskGroups[0].Tasks

	mainTask := tasks[0]
	sideTask := tasks[1]
	postTask := tasks[2]

	// Make the the third task a poststart hook
	postTask.Lifecycle.Hook = structs.TaskLifecycleHookPoststart

	coord := newTaskHookCoordinator(logger, tasks)
	postCh := coord.startConditionForTask(postTask)
	sideCh := coord.startConditionForTask(sideTask)
	mainCh := coord.startConditionForTask(mainTask)

	require.Truef(t, isChannelClosed(sideCh), "%s channel was open, should be closed", sideTask.Name)
	require.Falsef(t, isChannelClosed(mainCh), "%s channel was closed, should be open", mainTask.Name)
	require.Falsef(t, isChannelClosed(mainCh), "%s channel was closed, should be open", postTask.Name)

	states := map[string]*structs.TaskState{
		postTask.Name: {
			State:  structs.TaskStatePending,
			Failed: false,
		},
		mainTask.Name: {
			State:     structs.TaskStateRunning,
			Failed:    false,
			StartedAt: time.Now(),
		},
		sideTask.Name: {
			State:     structs.TaskStateRunning,
			Failed:    false,
			StartedAt: time.Now(),
		},
	}

	coord.taskStateUpdated(states)

	require.Truef(t, isChannelClosed(postCh), "%s channel was open, should be closed", postTask.Name)
	require.Truef(t, isChannelClosed(sideCh), "%s channel was open, should be closed", sideTask.Name)
	require.Truef(t, isChannelClosed(mainCh), "%s channel was open, should be closed", mainTask.Name)
}

func isChannelClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestHasSidecarTasks(t *testing.T) {

	falseV, trueV := false, true

	cases := []struct {
		name string
		// nil if main task, false if non-sidecar hook, true if sidecar hook
		indicators []*bool

		hasSidecars    bool
		hasNonsidecars bool
	}{
		{
			name:           "all sidecar - one",
			indicators:     []*bool{&trueV},
			hasSidecars:    true,
			hasNonsidecars: false,
		},
		{
			name:           "all sidecar - multiple",
			indicators:     []*bool{&trueV, &trueV, &trueV},
			hasSidecars:    true,
			hasNonsidecars: false,
		},
		{
			name:           "some sidecars, some others",
			indicators:     []*bool{nil, &falseV, &trueV},
			hasSidecars:    true,
			hasNonsidecars: true,
		},
		{
			name:           "no sidecars",
			indicators:     []*bool{nil, &falseV, nil},
			hasSidecars:    false,
			hasNonsidecars: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			alloc := allocWithSidecarIndicators(c.indicators)
			arConf, cleanup := testAllocRunnerConfig(t, alloc)
			defer cleanup()

			ar, err := NewAllocRunner(arConf)
			require.NoError(t, err)

			require.Equal(t, c.hasSidecars, hasSidecarTasks(ar.tasks), "sidecars")

			runners := []*taskrunner.TaskRunner{}
			for _, r := range ar.tasks {
				runners = append(runners, r)
			}
			require.Equal(t, c.hasNonsidecars, hasNonSidecarTasks(runners), "non-sidecars")

		})
	}
}

func allocWithSidecarIndicators(indicators []*bool) *structs.Allocation {
	alloc := mock.BatchAlloc()

	tasks := []*structs.Task{}
	resources := map[string]*structs.AllocatedTaskResources{}

	tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]

	for i, indicator := range indicators {
		task := alloc.Job.TaskGroups[0].Tasks[0].Copy()
		task.Name = fmt.Sprintf("task%d", i)
		if indicator != nil {
			task.Lifecycle = &structs.TaskLifecycleConfig{
				Hook:    structs.TaskLifecycleHookPrestart,
				Sidecar: *indicator,
			}
		}
		tasks = append(tasks, task)
		resources[task.Name] = tr
	}

	alloc.Job.TaskGroups[0].Tasks = tasks

	alloc.AllocatedResources.Tasks = resources
	return alloc

}
