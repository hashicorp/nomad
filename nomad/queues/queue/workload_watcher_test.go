// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

// testWorkload is safe for concurrent use so tests can inspect it while it
// is being watched.
type testWorkload struct {
	mu     sync.Mutex
	eval   *structs.Evaluation
	status string
}

func (w *testWorkload) ID() structs.NamespacedID {
	return structs.NewNamespacedID("", "")
}

func (w *testWorkload) JobVersion() uint64 {
	return 0
}

func (w *testWorkload) Eval() *structs.Evaluation {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.eval
}

func (w *testWorkload) SetEval(e *structs.Evaluation) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.eval = e
}

func (w *testWorkload) Status() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status
}

func (w *testWorkload) SetStatus(s, d string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = fmt.Sprintf("%s %s", s, d)
}

func TestWorkloadWatcher_WaitForPlacement(t *testing.T) {
	// newWorkloadAndJob creates a job in state and an eval that references it,
	// returning a testWorkload ready to be passed to WaitForPlacement.
	newWorkloadAndJob := func(t *testing.T, ss *state.StateStore) (*testWorkload, *structs.Job) {
		t.Helper()
		testEval := mock.Eval()
		job := mock.Job()
		job.Type = structs.JobTypeBatch
		job.ID = testEval.JobID
		job.Namespace = testEval.Namespace
		job.Placed = false
		must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 1, nil, job))
		must.NoError(t, ss.UpsertEvals(structs.MsgTypeTestSetup, 2, []*structs.Evaluation{testEval}))
		return &testWorkload{eval: testEval}, job
	}

	t.Run("returns when placed is set", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default())

		workload, job := newWorkloadAndJob(t, ss)

		doneCh := make(chan error, 1)
		go func() {
			doneCh <- watcher.WaitForPlacement(t.Context(), workload)
		}()

		// Give the goroutine time to enter the watch loop before triggering.
		must.Wait(t, wait.InitialSuccess(
			wait.BoolFunc(func() bool {
				return len(watcher.GetInProgressWorkloads()) == 1
			}),
			wait.Timeout(5*time.Second),
			wait.Gap(10*time.Millisecond),
		))

		select {
		case <-doneCh:
			t.Fatal("should not have returned yet")
		default:
		}

		// Mark the job as placed.
		placed := job.Copy()
		placed.Placed = true
		must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 3, nil, placed))

		t.Log("waiting for doneCh")

		must.NoError(t, <-doneCh)
		must.Eq(t, 0, len(watcher.GetInProgressWorkloads()))
	})

	t.Run("returns if job is removed from state", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default())

		workload, job := newWorkloadAndJob(t, ss)

		doneCh := make(chan error, 1)
		go func() {
			doneCh <- watcher.WaitForPlacement(t.Context(), workload)
		}()

		must.Wait(t, wait.InitialSuccess(
			wait.BoolFunc(func() bool {
				return len(watcher.GetInProgressWorkloads()) == 1
			}),
			wait.Timeout(5*time.Second),
			wait.Gap(10*time.Millisecond),
		))

		// Deregister the job so it disappears from state.
		must.NoError(t, ss.DeleteJob(3, job.Namespace, job.ID))

		must.NoError(t, <-doneCh)
		must.Eq(t, 0, len(watcher.GetInProgressWorkloads()))
	})
}

func TestWorkloadWatcher_isConstraintFailure(t *testing.T) {
	t.Run("detects constraint failure without resource exhaustion", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default())

		testEval := mock.Eval()
		testEval.FailedTGAllocs = map[string]*structs.AllocMetric{
			"web": {
				ConstraintFiltered: map[string]int{
					"${attr.kernel.name} == linux": 5,
				},
				NodesExhausted:     0,
				ClassExhausted:     map[string]int{},
				NodesAvailable:     map[string]int{"test": 15},
				DimensionExhausted: map[string]int{},
				QuotaExhausted:     []string{},
			},
		}

		workload := &testWorkload{eval: testEval}
		failure, reason := watcher.isConstraintFailure(workload)
		must.True(t, failure)
		must.Eq(t, reason, "${attr.kernel.name} == linux")
	})

	t.Run("does not detect constraint failure with resource exhaustion", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default())

		testEval := mock.Eval()
		testEval.FailedTGAllocs = map[string]*structs.AllocMetric{
			"web": {
				ConstraintFiltered: map[string]int{
					"${attr.kernel.name} == linux": 5,
				},
				NodesExhausted: 10,
				NodesAvailable: map[string]int{"test": 5},
				ClassExhausted: map[string]int{"compute": 5},
			},
		}

		workload := &testWorkload{eval: testEval}
		failure, _ := watcher.isConstraintFailure(workload)
		must.False(t, failure)
	})

	t.Run("detects pure resource exhaustion as not constraint failure", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default())

		testEval := mock.Eval()
		testEval.FailedTGAllocs = map[string]*structs.AllocMetric{
			"web": {
				ConstraintFiltered: map[string]int{},
				NodesExhausted:     15,
				NodesAvailable:     map[string]int{"test": 15},
				DimensionExhausted: map[string]int{"cpu": 10, "memory": 5},
			},
		}

		workload := &testWorkload{eval: testEval}
		failure, _ := watcher.isConstraintFailure(workload)
		must.False(t, failure)
	})

	t.Run("handles nil FailedTGAllocs", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default())

		testEval := mock.Eval()
		testEval.FailedTGAllocs = nil

		workload := &testWorkload{eval: testEval}
		failure, _ := watcher.isConstraintFailure(workload)
		must.False(t, failure)
	})

	t.Run("handles quota exhaustion as resource issue", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default())

		testEval := mock.Eval()
		testEval.FailedTGAllocs = map[string]*structs.AllocMetric{
			"web": {
				ConstraintFiltered: map[string]int{
					"${attr.kernel.name} == linux": 5,
				},
				QuotaExhausted: []string{"default"},
				NodesAvailable: map[string]int{"test": 15},
			},
		}

		workload := &testWorkload{eval: testEval}
		failure, _ := watcher.isConstraintFailure(workload)
		must.False(t, failure)
	})
}

func TestWorkloadWatcher_TrackPlacement(t *testing.T) {
	t.Run("tracks and untracks workloads", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default())

		testEval1 := mock.Eval()
		testEval2 := mock.Eval()
		workload1 := &testWorkload{eval: testEval1}
		workload2 := &testWorkload{eval: testEval2}

		inProgress := watcher.GetInProgressWorkloads()
		must.Eq(t, 0, len(inProgress))

		watcher.TrackPlacement(workload1)
		inProgress = watcher.GetInProgressWorkloads()
		must.Eq(t, 1, len(inProgress))
		must.NotNil(t, inProgress[testEval1.ID])

		watcher.TrackPlacement(workload2)
		inProgress = watcher.GetInProgressWorkloads()
		must.Eq(t, 2, len(inProgress))

		watcher.UntrackPlacement(workload1)
		inProgress = watcher.GetInProgressWorkloads()
		must.Eq(t, 1, len(inProgress))
		must.NotNil(t, inProgress[testEval2.ID])

		watcher.UntrackPlacement(workload2)
		inProgress = watcher.GetInProgressWorkloads()
		must.Eq(t, 0, len(inProgress))
	})
}
