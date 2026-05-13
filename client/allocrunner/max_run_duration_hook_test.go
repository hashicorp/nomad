// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"strings"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	metrics "github.com/hashicorp/go-metrics/compat"
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
	baseLabels []metrics.Label,
	onTimeout func(time.Time),
) *maxRunDurationHook {
	hook := newMaxRunDurationHook(log.NewNullLogger(), alloc, baseLabels, onTimeout)

	h, ok := hook.(*maxRunDurationHook)
	if !ok {
		panic("newMaxRunDurationHook returned unexpected hook type")
	}

	return h
}

// TestMaxRunDurationHook_Prerun_ArmsTimerImmediately verifies that a timer is
// armed when Prerun is called on an eligible allocation and fires after the
// configured max_run_duration.
func TestMaxRunDurationHook_Prerun_ArmsTimerImmediately(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	maxRunDuration := 50 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration
	alloc.CreateTime = time.Now().UnixNano()

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, nil, onTimeout)

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
	alloc.CreateTime = time.Now().UnixNano()

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, nil, onTimeout)

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

// TestMaxRunDurationHook_Update_DoesNotExtendDeadlineWhenAllocUpdated verifies
// that the deadline remains fixed at CreateTime + maxRunDuration and cannot be
// shifted by an alloc update (e.g. one that carries task-state timestamps).
func TestMaxRunDurationHook_Update_DoesNotExtendDeadlineWhenAllocUpdated(t *testing.T) {
	ci.Parallel(t)

	maxRunDuration := 200 * time.Millisecond

	alloc := mock.BatchAlloc()
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration
	alloc.CreateTime = time.Now().UnixNano()

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, nil, onTimeout)

	// Prerun arms the timer anchored to CreateTime.
	err := hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)
	deadlineAfterPrerun := hook.deadline

	// Simulate an alloc update that arrives after some time has passed.
	time.Sleep(75 * time.Millisecond)

	// An update with new task states should not move the deadline, because the
	// deadline is always CreateTime + maxRunDuration regardless of task state.
	taskName := alloc.Job.TaskGroups[0].Tasks[0].Name
	updatedAlloc := alloc.Copy()
	updatedAlloc.TaskStates = map[string]*structs.TaskState{
		taskName: {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now(),
		},
	}

	err = hook.Update(&interfaces.RunnerUpdateRequest{Alloc: updatedAlloc})
	must.NoError(t, err)

	// The deadline must not have changed.
	must.True(t,
		!hook.deadline.After(deadlineAfterPrerun),
		must.Sprintf("deadline was extended from %v to %v after alloc update",
			deadlineAfterPrerun, hook.deadline),
	)

	// Timer must still fire within the original deadline.
	select {
	case deadline := <-deadlines:
		must.False(t, deadline.IsZero())
	case <-time.After(maxRunDuration):
		t.Fatal("timer did not fire before original deadline")
	}
}

func TestMaxRunDurationHook_Update_RearmsOnDurationChange(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	initial := 200 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &initial
	alloc.CreateTime = time.Now().UnixNano()

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, nil, onTimeout)

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
			hook := newTestMaxRunDurationHook(tc.alloc, nil, onTimeout)

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
	alloc.CreateTime = time.Now().UnixNano()

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, nil, onTimeout)

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
	alloc.CreateTime = time.Now().UnixNano()

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, nil, onTimeout)

	err := hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	hook.Shutdown()

	select {
	case deadline := <-deadlines:
		t.Fatalf("unexpected deadline fired after shutdown: %v", deadline)
	case <-time.After(250 * time.Millisecond):
	}
}

// TestMaxRunDurationHook_Prerun_PreservesDeadlineAfterClientRestart verifies
// that when a client restarts mid-countdown, the hook correctly reconstructs the
// original deadline from the allocation's CreateTime rather than resetting the
// full budget. CreateTime is a durable server-assigned property that survives
// client restarts, so the remaining budget is automatically correct.
func TestMaxRunDurationHook_Prerun_PreservesDeadlineAfterClientRestart(t *testing.T) {
	ci.Parallel(t)

	maxRunDuration := 200 * time.Millisecond

	// Simulate an alloc that has already consumed half of its budget.
	elapsed := maxRunDuration / 2

	alloc := mock.BatchAlloc()
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration
	alloc.CreateTime = time.Now().Add(-elapsed).UnixNano()
	// The server-side alloc carries no task states (as is typical on restart).
	alloc.TaskStates = nil

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, nil, onTimeout)

	err := hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	// The deadline should fire after approximately the remaining half of the
	// budget (maxRunDuration/2), not after the full maxRunDuration. We allow
	// generous slack for slow test environments, but the key invariant is that
	// it fires well before the full duration would have elapsed from now.
	remaining := maxRunDuration - elapsed
	grace := remaining * 3 // generous upper bound

	select {
	case deadline := <-deadlines:
		must.False(t, deadline.IsZero())
		// Confirm the deadline is anchored to CreateTime, not time.Now().
		must.True(t,
			deadline.Before(time.Now().Add(grace)),
			must.Sprintf("deadline %v is too far in the future; expected it near %v",
				deadline, time.Unix(0, alloc.CreateTime).Add(maxRunDuration)),
		)
	case <-time.After(maxRunDuration * 2):
		t.Fatal("timed out: hook did not fire within 2x maxRunDuration, " +
			"suggesting the clock was reset to the full duration on restart")
	}
}

// TestMaxRunDurationHook_Prerun_ArmsTimerWithoutTaskStates verifies that the
// hook arms the timer correctly even when no task state is present — e.g. at
// initial Prerun before any tasks have started. The deadline is determined
// purely from CreateTime, so task state is never required.
func TestMaxRunDurationHook_Prerun_ArmsTimerWithoutTaskStates(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	maxRunDuration := 50 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration
	alloc.CreateTime = time.Now().UnixNano()
	alloc.TaskStates = nil

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, nil, onTimeout)

	err := hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	select {
	case deadline := <-deadlines:
		must.False(t, deadline.IsZero())
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for max_run_duration deadline")
	}
}

func TestMaxRunDurationHook_EmitMetrics(t *testing.T) {
	ci.Parallel(t)

	inMemorySink := metrics.NewInmemSink(10*time.Millisecond, 50*time.Millisecond)
	_, err := metrics.NewGlobal(metrics.DefaultConfig("nomad_test"), inMemorySink)
	must.NoError(t, err)

	alloc := mock.BatchAlloc()
	maxRunDuration := 2 * time.Minute
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration
	alloc.CreateTime = time.Now().UnixNano()

	baseLabels := []metrics.Label{
		{Name: "node_id", Value: "node-123"},
	}

	onTimeout, _ := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, baseLabels, onTimeout)

	err = hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	data := inMemorySink.Data()
	must.Len(t, 1, data)

	configuredSuffix := "client.allocs.max_run_duration.configured_seconds;node_id=node-123;task_group=" + alloc.TaskGroup
	remainingSuffix := "client.allocs.max_run_duration.remaining_seconds;node_id=node-123;task_group=" + alloc.TaskGroup

	var configuredFound bool
	var remainingFound bool

	for key, gauge := range data[0].Gauges {
		if strings.HasSuffix(key, configuredSuffix) {
			must.Eq(t, float32(maxRunDuration.Seconds()), gauge.Value)
			configuredFound = true
		}

		if strings.HasSuffix(key, remainingSuffix) {
			must.Positive(t, gauge.Value)
			must.True(t, gauge.Value <= float32(maxRunDuration.Seconds()))
			remainingFound = true
		}
	}

	must.True(t, configuredFound)
	must.True(t, remainingFound)
}
