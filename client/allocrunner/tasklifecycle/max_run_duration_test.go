// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package tasklifecycle

import (
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

type testMaxRunDurationSetter struct {
	deadlines chan time.Time
}

func newTestMaxRunDurationSetter() *testMaxRunDurationSetter {
	return &testMaxRunDurationSetter{
		deadlines: make(chan time.Time, 8),
	}
}

func (s *testMaxRunDurationSetter) EnforceMaxRunDurationTimeout(deadline time.Time) {
	s.deadlines <- deadline
}

func TestMaxRunDuration_FullyRunningSince(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	earlier := now.Add(-2 * time.Second)
	later := now.Add(-1 * time.Second)

	got, ok := FullyRunningSince(map[string]*structs.TaskState{
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

func TestMaxRunDuration_FullyRunningSince_FalseWhenEmpty(t *testing.T) {
	t.Parallel()

	got, ok := FullyRunningSince(nil)
	must.False(t, ok)
	must.Eq(t, time.Time{}, got)
}

func TestMaxRunDuration_FullyRunningSince_FalseWhenTaskNotRunning(t *testing.T) {
	t.Parallel()

	_, ok := FullyRunningSince(map[string]*structs.TaskState{
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

func TestMaxRunDuration_FullyRunningSince_FalseWhenStartedAtMissing(t *testing.T) {
	t.Parallel()

	_, ok := FullyRunningSince(map[string]*structs.TaskState{
		"a": {
			State: structs.TaskStateRunning,
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

	setter := newTestMaxRunDurationSetter()
	m := NewMaxRunDuration(log.NewNullLogger(), alloc, setter)

	startedAt := time.Now().UTC()
	m.TaskStateUpdated(map[string]*structs.TaskState{
		"web": {
			State:     structs.TaskStateRunning,
			StartedAt: startedAt,
		},
	})

	select {
	case deadline := <-setter.deadlines:
		must.Eq(t, startedAt.Add(maxRunDuration), deadline)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for max_run_duration deadline")
	}
}

func TestMaxRunDuration_TaskStateUpdated_ImmediateEnforcementWhenExpired(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	maxRunDuration := 25 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration

	setter := newTestMaxRunDurationSetter()
	m := NewMaxRunDuration(log.NewNullLogger(), alloc, setter)

	startedAt := time.Now().Add(-2 * maxRunDuration).UTC()
	m.TaskStateUpdated(map[string]*structs.TaskState{
		"web": {
			State:     structs.TaskStateRunning,
			StartedAt: startedAt,
		},
	})

	select {
	case deadline := <-setter.deadlines:
		must.Eq(t, startedAt.Add(maxRunDuration), deadline)
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for immediate max_run_duration enforcement")
	}
}

func TestMaxRunDuration_TaskStateUpdated_DoesNotFireWhenNotFullyRunning(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	maxRunDuration := 25 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration

	setter := newTestMaxRunDurationSetter()
	m := NewMaxRunDuration(log.NewNullLogger(), alloc, setter)

	m.TaskStateUpdated(map[string]*structs.TaskState{
		"web": {
			State: structs.TaskStatePending,
		},
	})

	select {
	case deadline := <-setter.deadlines:
		t.Fatalf("unexpected deadline fired: %v", deadline)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestMaxRunDuration_TaskStateUpdated_DoesNotFireForNonBatchJobs(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	maxRunDuration := 25 * time.Millisecond
	alloc.Job.Type = structs.JobTypeService
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration

	setter := newTestMaxRunDurationSetter()
	m := NewMaxRunDuration(log.NewNullLogger(), alloc, setter)

	m.TaskStateUpdated(map[string]*structs.TaskState{
		"web": {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now().UTC(),
		},
	})

	select {
	case deadline := <-setter.deadlines:
		t.Fatalf("unexpected deadline fired: %v", deadline)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestMaxRunDuration_TaskStateUpdated_DoesNotFireWhenDesiredStatusNotRun(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	maxRunDuration := 25 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration
	alloc.DesiredStatus = structs.AllocDesiredStatusStop

	setter := newTestMaxRunDurationSetter()
	m := NewMaxRunDuration(log.NewNullLogger(), alloc, setter)

	m.TaskStateUpdated(map[string]*structs.TaskState{
		"web": {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now().UTC(),
		},
	})

	select {
	case deadline := <-setter.deadlines:
		t.Fatalf("unexpected deadline fired: %v", deadline)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestMaxRunDuration_TaskStateUpdated_DoesNotFireWhenAllocTerminal(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	maxRunDuration := 25 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration
	alloc.ClientStatus = structs.AllocClientStatusComplete

	setter := newTestMaxRunDurationSetter()
	m := NewMaxRunDuration(log.NewNullLogger(), alloc, setter)

	m.TaskStateUpdated(map[string]*structs.TaskState{
		"web": {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now().UTC(),
		},
	})

	select {
	case deadline := <-setter.deadlines:
		t.Fatalf("unexpected deadline fired: %v", deadline)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestMaxRunDuration_SetAlloc_UsesLatestAllocConfig(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	initial := 200 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &initial

	setter := newTestMaxRunDurationSetter()
	m := NewMaxRunDuration(log.NewNullLogger(), alloc, setter)

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
	case deadline := <-setter.deadlines:
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

	setter := newTestMaxRunDurationSetter()
	m := NewMaxRunDuration(log.NewNullLogger(), alloc, setter)

	m.TaskStateUpdated(map[string]*structs.TaskState{
		"web": {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now().UTC(),
		},
	})

	m.Stop()

	select {
	case deadline := <-setter.deadlines:
		t.Fatalf("unexpected deadline fired after stop: %v", deadline)
	case <-time.After(250 * time.Millisecond):
	}
}
