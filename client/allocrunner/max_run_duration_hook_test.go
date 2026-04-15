// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func newTestMaxRunDurationHookCallback() (func(time.Time), chan time.Time) {
	deadlines := make(chan time.Time, 8)
	return func(deadline time.Time) {
		deadlines <- deadline
	}, deadlines
}

func newTestMaxRunDurationHook(
	alloc *structs.Allocation,
	onTimeout func(time.Time),
) *maxRunDurationHook {
	hook := newMaxRunDurationHook(log.NewNullLogger(), alloc, onTimeout)

	h, ok := hook.(*maxRunDurationHook)
	if !ok {
		panic("newMaxRunDurationHook returned unexpected hook type")
	}

	return h
}

func TestMaxRunDurationHook_Prerun_ArmsTimerImmediately(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	maxRunDuration := 50 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, onTimeout)

	err := hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	select {
	case deadline := <-deadlines:
		must.False(t, deadline.IsZero())
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for max_run_duration deadline")
	}
}

func TestMaxRunDurationHook_Update_DoesNotExtendDeadlineOnUnrelatedAllocChange(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	maxRunDuration := 50 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, onTimeout)

	err := hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	updated := alloc.Copy()
	updated.ClientDescription = "unrelated alloc update"

	time.Sleep(20 * time.Millisecond)

	err = hook.Update(&interfaces.RunnerUpdateRequest{Alloc: updated})
	must.NoError(t, err)

	select {
	case <-deadlines:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for original max_run_duration deadline after unrelated update")
	}
}

func TestMaxRunDurationHook_Update_RearmsOnDurationChange(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	initial := 200 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &initial

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, onTimeout)

	err := hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	updated := alloc.Copy()
	latest := 40 * time.Millisecond
	updated.Job.TaskGroups[0].MaxRunDuration = &latest

	err = hook.Update(&interfaces.RunnerUpdateRequest{Alloc: updated})
	must.NoError(t, err)

	select {
	case <-deadlines:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for updated max_run_duration deadline")
	}
}

func TestMaxRunDurationHook_TaskLevelOverride_DisablesAllocTimer(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	groupMaxRunDuration := 25 * time.Millisecond
	taskMaxRunDuration := 50 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &groupMaxRunDuration
	alloc.Job.TaskGroups[0].Tasks[0].MaxRunDuration = &taskMaxRunDuration

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, onTimeout)

	err := hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	select {
	case deadline := <-deadlines:
		t.Fatalf("unexpected alloc-level deadline fired with task-level override: %v", deadline)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestMaxRunDurationHook_TaskLevelOverride_DisablesAllocTimerWithMultipleTaskOverrides(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	groupMaxRunDuration := 25 * time.Millisecond
	taskOneMaxRunDuration := 50 * time.Millisecond
	taskTwoMaxRunDuration := 60 * time.Millisecond

	mainTask := alloc.Job.TaskGroups[0].Tasks[0]
	secondTask := mainTask.Copy()
	secondTask.Name = "worker"

	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &groupMaxRunDuration
	mainTask.MaxRunDuration = &taskOneMaxRunDuration
	secondTask.MaxRunDuration = &taskTwoMaxRunDuration
	alloc.Job.TaskGroups[0].Tasks = []*structs.Task{mainTask, secondTask}

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, onTimeout)

	err := hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	select {
	case deadline := <-deadlines:
		t.Fatalf("unexpected alloc-level deadline fired with multiple task-level overrides: %v", deadline)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestMaxRunDurationHook_DoesNotFireWhenAllocNotEligible(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name  string
		alloc *structs.Allocation
	}{
		{
			name: "non-batch job",
			alloc: func() *structs.Allocation {
				alloc := mock.BatchAlloc()
				maxRunDuration := 25 * time.Millisecond
				alloc.Job.Type = structs.JobTypeService
				alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration
				return alloc
			}(),
		},
		{
			name: "desired status not run",
			alloc: func() *structs.Allocation {
				alloc := mock.BatchAlloc()
				maxRunDuration := 25 * time.Millisecond
				alloc.Job.Type = structs.JobTypeBatch
				alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration
				alloc.DesiredStatus = structs.AllocDesiredStatusStop
				return alloc
			}(),
		},
		{
			name: "terminal alloc",
			alloc: func() *structs.Allocation {
				alloc := mock.BatchAlloc()
				maxRunDuration := 25 * time.Millisecond
				alloc.Job.Type = structs.JobTypeBatch
				alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration
				alloc.ClientStatus = structs.AllocClientStatusComplete
				return alloc
			}(),
		},
		{
			name: "no max run duration",
			alloc: func() *structs.Allocation {
				alloc := mock.BatchAlloc()
				alloc.Job.Type = structs.JobTypeBatch
				alloc.Job.TaskGroups[0].MaxRunDuration = nil
				return alloc
			}(),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			onTimeout, deadlines := newTestMaxRunDurationHookCallback()
			hook := newTestMaxRunDurationHook(tc.alloc, onTimeout)

			err := hook.Prerun((*taskenv.TaskEnv)(nil))
			must.NoError(t, err)

			select {
			case deadline := <-deadlines:
				t.Fatalf("unexpected deadline fired: %v", deadline)
			case <-time.After(100 * time.Millisecond):
			}
		})
	}
}

func TestMaxRunDurationHook_Postrun_CancelsTimer(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	maxRunDuration := 150 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, onTimeout)

	err := hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	err = hook.Postrun()
	must.NoError(t, err)

	select {
	case deadline := <-deadlines:
		t.Fatalf("unexpected deadline fired after postrun: %v", deadline)
	case <-time.After(250 * time.Millisecond):
	}
}

func TestMaxRunDurationHook_Shutdown_CancelsTimer(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	maxRunDuration := 150 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, onTimeout)

	err := hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	hook.Shutdown()

	select {
	case deadline := <-deadlines:
		t.Fatalf("unexpected deadline fired after shutdown: %v", deadline)
	case <-time.After(250 * time.Millisecond):
	}
}
