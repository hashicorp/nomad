package allocrunner

import (
	"testing"

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

	select {
	case _, ok := <-ch:
		require.False(t, ok)
	default:
		require.Fail(t, "channel wasn't closed")
	}
}

func TestTaskHookCoordinator_Prestart(t *testing.T) {
	alloc := mock.Alloc()
	tasks := alloc.Job.TaskGroups[0].Tasks
	logger := testlog.HCLogger(t)

	tasks = append(tasks, mock.InitTask())
	tasks = append(tasks, mock.SidecarTask())
	coord := newTaskHookCoordinator(logger, tasks)

	mainCh := coord.startConditionForTask(tasks[0])
	initCh := coord.startConditionForTask(tasks[1])
	sideCh := coord.startConditionForTask(tasks[2])

	select {
	case _, ok := <-initCh:
		require.False(t, ok)
	case _, ok := <-sideCh:
		require.False(t, ok)
	default:
		require.Fail(t, "prestart channels weren't closed")
	}

	select {
	case <-mainCh:
		require.Fail(t, "channel was closed, should be open")
	default:
		// channel for main task is open, which is correct: coordinator should
		// block all other tasks until prestart tasks are completed
	}
}
