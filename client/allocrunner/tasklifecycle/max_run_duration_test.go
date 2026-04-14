// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package tasklifecycle

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func newTestMaxRunDurationCallback() (func(time.Time), chan time.Time) {
	deadlines := make(chan time.Time, 8)
	return func(deadline time.Time) {
		deadlines <- deadline
	}, deadlines
}

func TestMaxRunDuration_FullyRunningSince(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	earlier := now.Add(-2 * time.Second)
	later := now.Add(-1 * time.Second)

	got, ok := structs.FullyRunningSince(map[string]*structs.TaskState{
		"a": {
			State:     structs.TaskStateRunning,
			StartedAt: earlier,
		},
		"b": {
			State:     structs.TaskStateRunning,
			StartedAt: later,
		},
	})

	must.True(t, ok)
	must.Eq(t, later, got)
}

func TestMaxRunDuration_FullyRunningSince_FalseWhenNotFullyRunning(t *testing.T) {
	t.Parallel()

	_, ok := structs.FullyRunningSince(map[string]*structs.TaskState{
		"a": {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now().Add(-time.Second).UTC(),
		},
		"b": {
			State: structs.TaskStatePending,
		},
	})

	must.False(t, ok)
}

func TestMaxRunDuration_TaskStateUpdated_ArmsTimerAndFires(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	maxRunDuration := 50 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration

	onTimeout, deadlines := newTestMaxRunDurationCallback()
	m := NewMaxRunDuration(onTimeout)
	m.SetAlloc(alloc)

	startedAt := time.Now().UTC()
	m.TaskStateUpdated(map[string]*structs.TaskState{
		"web": {
			State:     structs.TaskStateRunning,
			StartedAt: startedAt,
		},
	})

	select {
	case deadline := <-deadlines:
		must.Eq(t, startedAt.Add(maxRunDuration), deadline)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for max_run_duration deadline")
	}
}

func TestMaxRunDuration_TaskStateUpdated_PreservesDeadlineWhenOneTaskFinishesEarly(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	maxRunDuration := 50 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration

	task2 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "web2"
	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2)

	onTimeout, deadlines := newTestMaxRunDurationCallback()
	m := NewMaxRunDuration(onTimeout)
	m.SetAlloc(alloc)

	startedAt := time.Now().UTC()
	m.TaskStateUpdated(map[string]*structs.TaskState{
		"web": {
			State:     structs.TaskStateRunning,
			StartedAt: startedAt,
		},
		"web2": {
			State:     structs.TaskStateRunning,
			StartedAt: startedAt,
		},
	})

	m.TaskStateUpdated(map[string]*structs.TaskState{
		"web": {
			State:     structs.TaskStateRunning,
			StartedAt: startedAt,
		},
		"web2": {
			State:      structs.TaskStateDead,
			StartedAt:  startedAt,
			FinishedAt: startedAt.Add(2 * time.Millisecond),
		},
	})

	select {
	case deadline := <-deadlines:
		must.Eq(t, startedAt.Add(maxRunDuration), deadline)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for max_run_duration deadline after one task finished early")
	}
}

func TestMaxRunDuration_TaskStateUpdated_DoesNotFireWhenAllocNotEligible(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		alloc *structs.Allocation
	}{
		{
			name: "not fully running",
			alloc: func() *structs.Allocation {
				alloc := mock.BatchAlloc()
				maxRunDuration := 25 * time.Millisecond
				alloc.Job.Type = structs.JobTypeBatch
				alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration
				return alloc
			}(),
		},
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			onTimeout, deadlines := newTestMaxRunDurationCallback()
			m := NewMaxRunDuration(onTimeout)
			m.SetAlloc(tc.alloc)

			taskStates := map[string]*structs.TaskState{
				"web": {
					State:     structs.TaskStateRunning,
					StartedAt: time.Now().UTC(),
				},
			}
			if tc.name == "not fully running" {
				taskStates["web"] = &structs.TaskState{State: structs.TaskStatePending}
			}

			m.TaskStateUpdated(taskStates)

			select {
			case deadline := <-deadlines:
				t.Fatalf("unexpected deadline fired: %v", deadline)
			case <-time.After(100 * time.Millisecond):
			}
		})
	}
}

func TestMaxRunDuration_SetAlloc_UsesLatestAllocConfig(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	initial := 200 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &initial

	onTimeout, deadlines := newTestMaxRunDurationCallback()
	m := NewMaxRunDuration(onTimeout)
	m.SetAlloc(alloc)

	updated := alloc.Copy()
	latest := 40 * time.Millisecond
	updated.Job.TaskGroups[0].MaxRunDuration = &latest
	m.SetAlloc(updated)

	startedAt := time.Now().UTC()
	m.TaskStateUpdated(map[string]*structs.TaskState{
		"web": {
			State:     structs.TaskStateRunning,
			StartedAt: startedAt,
		},
	})

	select {
	case deadline := <-deadlines:
		must.Eq(t, startedAt.Add(latest), deadline)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for updated max_run_duration deadline")
	}
}

func TestMaxRunDuration_Stop_CancelsTimer(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	maxRunDuration := 150 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration

	onTimeout, deadlines := newTestMaxRunDurationCallback()
	m := NewMaxRunDuration(onTimeout)
	m.SetAlloc(alloc)

	m.TaskStateUpdated(map[string]*structs.TaskState{
		"web": {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now().UTC(),
		},
	})

	m.Stop()

	select {
	case deadline := <-deadlines:
		t.Fatalf("unexpected deadline fired after stop: %v", deadline)
	case <-time.After(250 * time.Millisecond):
	}
}
