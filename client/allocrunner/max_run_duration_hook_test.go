// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
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

func TestMaxRunDurationHook_Prerun_ArmsTimerImmediately(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	maxRunDuration := 50 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration

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

func TestMaxRunDurationHook_Update_RearmsOnDurationChange(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	initial := 200 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &initial

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

func TestMaxRunDurationHook_Prerun_ArmsTimerBeforeTasksAreFullyRunning(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	maxRunDuration := 50 * time.Millisecond
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].MaxRunDuration = &maxRunDuration

	// Simulate the initial prerun state where tasks have not yet started and
	// therefore no StartedAt timestamps are available to compute a fully-running
	// deadline from task state.
	for _, ts := range alloc.TaskStates {
		ts.State = structs.TaskStatePending
		ts.StartedAt = time.Time{}
	}

	onTimeout, deadlines := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, nil, onTimeout)

	err := hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	select {
	case deadline := <-deadlines:
		must.False(t, deadline.IsZero())
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for max_run_duration deadline before tasks were fully running")
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

	baseLabels := []metrics.Label{
		{Name: "node_id", Value: "node-123"},
	}

	onTimeout, _ := newTestMaxRunDurationHookCallback()
	hook := newTestMaxRunDurationHook(alloc, baseLabels, onTimeout)

	err = hook.Prerun((*taskenv.TaskEnv)(nil))
	must.NoError(t, err)

	data := inMemorySink.Data()

	var configuredFound bool
	for _, interval := range data {
		for _, gauge := range interval.Gauges {
			if gauge.Name != "nomad_test.client.allocs.max_run_duration.configured_seconds" {
				continue
			}

			labels := make(map[string]string, len(gauge.Labels))
			for _, label := range gauge.Labels {
				labels[label.Name] = label.Value
			}

			if labels["node_id"] == "node-123" && labels["task_group"] == alloc.TaskGroup {
				must.Eq(t, float32(maxRunDuration.Seconds()), gauge.Value)
				configuredFound = true
			}
		}
	}
	must.True(t, configuredFound)

	var remainingFound bool
	for _, interval := range data {
		for _, gauge := range interval.Gauges {
			if gauge.Name != "nomad_test.client.allocs.max_run_duration.remaining_seconds" {
				continue
			}

			labels := make(map[string]string, len(gauge.Labels))
			for _, label := range gauge.Labels {
				labels[label.Name] = label.Value
			}

			if labels["node_id"] == "node-123" && labels["task_group"] == alloc.TaskGroup {
				must.Positive(t, gauge.Value)
				must.LessEq(t, gauge.Value, float32(maxRunDuration.Seconds()))
				remainingFound = true
			}
		}
	}
	must.True(t, remainingFound)
}
