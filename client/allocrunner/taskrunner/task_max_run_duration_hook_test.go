// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	arinterfaces "github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func newTestTaskMaxRunDurationHook(tr *TaskRunner) *taskMaxRunDurationHook {
	hook := newTaskMaxRunDurationHook(tr, log.NewNullLogger())

	h, ok := hook.(*taskMaxRunDurationHook)
	if !ok {
		panic("newTaskMaxRunDurationHook returned unexpected hook type")
	}

	return h
}

func newTestTaskRunnerForMaxRunDurationHook(alloc *structs.Allocation, taskName string) *TaskRunner {
	task := alloc.LookupTask(taskName)
	if task == nil {
		panic("allocation missing task")
	}

	return &TaskRunner{
		allocID:  taskName + "-alloc",
		taskName: taskName,
		alloc:    alloc,
		task:     task,
		state:    &structs.TaskState{},
		clientConfig: &config.Config{
			MaxKillTimeout: 30 * time.Second,
		},
		killCtx: context.Background(),
		waitCh:  make(chan struct{}),
		logger:  log.NewNullLogger(),
	}
}

func TestTaskMaxRunDurationHook_Prestart_DoesNotArmBeforeTaskStarts(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	maxRunDuration := 40 * time.Millisecond
	task.MaxRunDuration = &maxRunDuration

	tr := newTestTaskRunnerForMaxRunDurationHook(alloc, task.Name)
	hook := newTestTaskMaxRunDurationHook(tr)

	err := hook.Prestart(context.Background(), &arinterfaces.TaskPrestartRequest{}, &arinterfaces.TaskPrestartResponse{})
	must.NoError(t, err)

	hook.mu.Lock()
	defer hook.mu.Unlock()

	must.Nil(t, hook.timer)
	must.False(t, hook.hasMaxRunDuration)
	must.True(t, hook.deadline.IsZero())
}

func TestTaskMaxRunDurationHook_Update_ArmsWhenTaskStartsRunning(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	maxRunDuration := 200 * time.Millisecond
	task.MaxRunDuration = &maxRunDuration

	tr := newTestTaskRunnerForMaxRunDurationHook(alloc, task.Name)
	hook := newTestTaskMaxRunDurationHook(tr)

	err := hook.Prestart(context.Background(), &arinterfaces.TaskPrestartRequest{}, &arinterfaces.TaskPrestartResponse{})
	must.NoError(t, err)

	startedAt := time.Now()
	tr.state = &structs.TaskState{
		State:     structs.TaskStateRunning,
		StartedAt: startedAt,
	}

	updated := alloc.Copy()
	err = hook.Update(context.Background(), &arinterfaces.TaskUpdateRequest{Alloc: updated}, &arinterfaces.TaskUpdateResponse{})
	must.NoError(t, err)

	hook.mu.Lock()
	defer hook.mu.Unlock()

	must.NotNil(t, hook.timer)
	must.True(t, hook.hasMaxRunDuration)
	must.Eq(t, maxRunDuration, hook.maxRunDuration)
	must.False(t, hook.deadline.IsZero())
	must.True(t, hook.deadline.After(startedAt))
}

func TestTaskMaxRunDurationHook_Update_RearmsOnDurationChange(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	initial := 200 * time.Millisecond
	task.MaxRunDuration = &initial

	tr := newTestTaskRunnerForMaxRunDurationHook(alloc, task.Name)
	hook := newTestTaskMaxRunDurationHook(tr)

	startedAt := time.Now()
	tr.state = &structs.TaskState{
		State:     structs.TaskStateRunning,
		StartedAt: startedAt,
	}

	err := hook.Prestart(context.Background(), &arinterfaces.TaskPrestartRequest{}, &arinterfaces.TaskPrestartResponse{})
	must.NoError(t, err)

	hook.mu.Lock()
	initialDeadline := hook.deadline
	hook.mu.Unlock()

	updated := alloc.Copy()
	latest := 50 * time.Millisecond
	updated.LookupTask(task.Name).MaxRunDuration = &latest

	err = hook.Update(context.Background(), &arinterfaces.TaskUpdateRequest{Alloc: updated}, &arinterfaces.TaskUpdateResponse{})
	must.NoError(t, err)

	hook.mu.Lock()
	defer hook.mu.Unlock()

	must.NotNil(t, hook.timer)
	must.Eq(t, latest, hook.maxRunDuration)
	must.True(t, hook.deadline.Before(initialDeadline))
}

func TestTaskMaxRunDurationHook_Update_DoesNotRearmOnUnrelatedAllocChange(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	maxRunDuration := 200 * time.Millisecond
	task.MaxRunDuration = &maxRunDuration

	tr := newTestTaskRunnerForMaxRunDurationHook(alloc, task.Name)
	hook := newTestTaskMaxRunDurationHook(tr)

	startedAt := time.Now()
	tr.state = &structs.TaskState{
		State:     structs.TaskStateRunning,
		StartedAt: startedAt,
	}

	err := hook.Prestart(context.Background(), &arinterfaces.TaskPrestartRequest{}, &arinterfaces.TaskPrestartResponse{})
	must.NoError(t, err)

	hook.mu.Lock()
	initialTimer := hook.timer
	initialDeadline := hook.deadline
	hook.mu.Unlock()

	updated := alloc.Copy()
	updated.ClientDescription = "unrelated alloc update"

	err = hook.Update(context.Background(), &arinterfaces.TaskUpdateRequest{Alloc: updated}, &arinterfaces.TaskUpdateResponse{})
	must.NoError(t, err)

	hook.mu.Lock()
	defer hook.mu.Unlock()

	must.Eq(t, initialTimer, hook.timer)
	must.Eq(t, initialDeadline, hook.deadline)
	must.Eq(t, maxRunDuration, hook.maxRunDuration)
}

func TestTaskMaxRunDurationHook_Exited_CancelsTimer(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	maxRunDuration := 150 * time.Millisecond
	task.MaxRunDuration = &maxRunDuration

	tr := newTestTaskRunnerForMaxRunDurationHook(alloc, task.Name)
	hook := newTestTaskMaxRunDurationHook(tr)

	tr.state = &structs.TaskState{
		State:     structs.TaskStateRunning,
		StartedAt: time.Now(),
	}

	err := hook.Prestart(context.Background(), &arinterfaces.TaskPrestartRequest{}, &arinterfaces.TaskPrestartResponse{})
	must.NoError(t, err)

	hook.mu.Lock()
	must.NotNil(t, hook.timer)
	hook.mu.Unlock()

	err = hook.Exited(context.Background(), &arinterfaces.TaskExitedRequest{}, &arinterfaces.TaskExitedResponse{})
	must.NoError(t, err)

	hook.mu.Lock()
	defer hook.mu.Unlock()

	must.Nil(t, hook.timer)
}

func TestTaskMaxRunDurationHook_CurrentDeadline_IgnoresNonRunningTask(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	maxRunDuration := 100 * time.Millisecond
	task.MaxRunDuration = &maxRunDuration

	tr := newTestTaskRunnerForMaxRunDurationHook(alloc, task.Name)
	tr.state = &structs.TaskState{
		State: structs.TaskStatePending,
	}

	hook := newTestTaskMaxRunDurationHook(tr)

	deadline, duration, ok := hook.currentDeadline()
	must.False(t, ok)
	must.True(t, deadline.IsZero())
	must.Zero(t, duration)
}

func TestTaskMaxRunDurationHook_CurrentDeadline_UsesTaskStartedAt(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	maxRunDuration := 100 * time.Millisecond
	task.MaxRunDuration = &maxRunDuration

	startedAt := time.Now().Add(-25 * time.Millisecond)

	tr := newTestTaskRunnerForMaxRunDurationHook(alloc, task.Name)
	tr.state = &structs.TaskState{
		State:     structs.TaskStateRunning,
		StartedAt: startedAt,
	}

	hook := newTestTaskMaxRunDurationHook(tr)

	deadline, duration, ok := hook.currentDeadline()
	must.True(t, ok)
	must.Eq(t, maxRunDuration, duration)
	must.Eq(t, startedAt.Add(maxRunDuration), deadline)
}

func TestTaskMaxRunDurationHook_CurrentDeadline_IgnoresTerminalAlloc(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	alloc.ClientStatus = structs.AllocClientStatusComplete

	task := alloc.Job.TaskGroups[0].Tasks[0]
	maxRunDuration := 100 * time.Millisecond
	task.MaxRunDuration = &maxRunDuration

	tr := newTestTaskRunnerForMaxRunDurationHook(alloc, task.Name)
	tr.state = &structs.TaskState{
		State:     structs.TaskStateRunning,
		StartedAt: time.Now(),
	}

	hook := newTestTaskMaxRunDurationHook(tr)

	deadline, duration, ok := hook.currentDeadline()
	must.False(t, ok)
	must.True(t, deadline.IsZero())
	must.Zero(t, duration)
}
