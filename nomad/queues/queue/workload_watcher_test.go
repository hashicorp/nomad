// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

type testWorkload struct {
	eval *structs.Evaluation
	wait bool
	s    string
}

func (w *testWorkload) GetEval() *structs.Evaluation {
	return w.eval
}
func (w *testWorkload) SetEval(e *structs.Evaluation) {
	w.eval = e
}
func (w *testWorkload) GetStatus() string {
	return w.s
}
func (w *testWorkload) SetStatus(s, d string) {
	w.s = fmt.Sprintf("%s %s", s, d)

}
func (w *testWorkload) WaitOnRestore() bool {
	return w.wait
}

func TestWorkloadWatcher_WaitForPlacement(t *testing.T) {
	t.Run("returns if eval complete", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})
		resultsCh := watcher.Results()

		testEval := mock.Eval()
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		ws := memdb.NewWatchSet()
		workload := &testWorkload{eval: testEval.Copy()}
		watcher.WaitForPlacement(t.Context(), workload, ws)

		inProgress := watcher.GetInProgressWorkloads()
		must.Eq(t, 1, len(inProgress))
		must.NotNil(t, inProgress[testEval.ID])

		testEval.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		result := <-resultsCh

		inProgress = watcher.GetInProgressWorkloads()
		must.Eq(t, 0, len(inProgress))

		must.NoError(t, result.Err)
	})

	t.Run("continues watching blocked evals", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})
		resultsCh := watcher.Results()

		testEval := mock.Eval()
		blocked := mock.Eval()

		testEval.Status = structs.EvalStatusComplete
		testEval.BlockedEval = blocked.ID

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval, blocked})

		ws := memdb.NewWatchSet()
		workload := &testWorkload{eval: testEval.Copy()}
		watcher.WaitForPlacement(t.Context(), workload, ws)

		// We want to make sure the testQueue has begun a watch on the blocked eval
		// before continuing, which is indicated by the length of the watchset being >0.
		must.Wait(t, wait.InitialSuccess(
			wait.ErrorFunc(func() error {
				if len(ws) == 0 {
					return fmt.Errorf("blocking query not started yet")
				}
				return nil
			}),
			wait.Timeout(5*time.Second),
			wait.Gap(100*time.Millisecond),
		))

		select {
		case <-resultsCh:
			t.Fatal("should not have exited")
		default:
		}

		blocked.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{blocked})

		result := <-resultsCh
		must.NoError(t, result.Err)

	})

	t.Run("continues watching next evals after eval failure", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})
		resultsCh := watcher.Results()

		testEval := mock.Eval()
		next := mock.Eval()

		testEval.Status = structs.EvalStatusFailed
		testEval.NextEval = next.ID

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval, next})

		ws := memdb.NewWatchSet()
		workload := &testWorkload{eval: testEval.Copy()}
		watcher.WaitForPlacement(t.Context(), workload, ws)

		// We want to make sure the testQueue has begun a watch on the blocked eval
		// before continuing, which is indicated by the length of the watchset being >0.
		must.Wait(t, wait.InitialSuccess(
			wait.ErrorFunc(func() error {
				if len(ws) == 0 {
					return fmt.Errorf("blocking query not started yet")
				}
				return nil
			}),
			wait.Timeout(5*time.Second),
			wait.Gap(100*time.Millisecond),
		))

		select {
		case <-resultsCh:
			t.Fatal("should not have exited")
		default:
		}

		next.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{next})

		result := <-resultsCh
		must.NoError(t, result.Err)
	})

	t.Run("returns deadline exceeded, still waits for completion", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{
			WorkloadTimeout:      10,
			ConcurrentPlacements: 1,
		})
		resultsCh := watcher.Results()

		testEval := mock.Eval()
		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval})

		ws := memdb.NewWatchSet()
		workload := &testWorkload{eval: testEval.Copy()}
		watcher.WaitForPlacement(t.Context(), workload, ws)
		must.False(t, watcher.CanAttemptPlacement())

		inProgress := watcher.GetInProgressWorkloads()
		must.Eq(t, 1, len(inProgress))
		must.NotNil(t, inProgress[testEval.ID])

		result := <-resultsCh

		must.True(t, result.TimedOut)
		must.False(t, watcher.CanAttemptPlacement())

		inProgress = watcher.GetInProgressWorkloads()
		must.Eq(t, 1, len(inProgress))
		must.NotNil(t, inProgress[testEval.ID])

		testEval.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		result = <-resultsCh

		must.False(t, result.TimedOut)
		must.True(t, watcher.CanAttemptPlacement())

		inProgress = watcher.GetInProgressWorkloads()
		must.Eq(t, 0, len(inProgress))
	})

	t.Run("tracks concurrent placements", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{
			WorkloadTimeout:      10,
			ConcurrentPlacements: 2,
		})
		resultsCh := watcher.Results()

		must.True(t, watcher.CanAttemptPlacement())

		testEval1 := mock.Eval()
		testEval2 := mock.Eval()
		testEval3 := mock.Eval()
		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval1, testEval2, testEval3})

		ws1 := memdb.NewWatchSet()
		ws2 := memdb.NewWatchSet()
		workload1 := &testWorkload{eval: testEval1.Copy()}
		workload2 := &testWorkload{eval: testEval2.Copy()}

		watcher.WaitForPlacement(t.Context(), workload1, ws1)
		time.Sleep(10 * time.Millisecond)
		must.True(t, watcher.CanAttemptPlacement())

		watcher.WaitForPlacement(t.Context(), workload2, ws2)
		time.Sleep(10 * time.Millisecond)
		must.False(t, watcher.CanAttemptPlacement())

		testEval1.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval1})

		result1 := <-resultsCh
		must.NoError(t, result1.Err)

		time.Sleep(10 * time.Millisecond)
		must.True(t, watcher.CanAttemptPlacement())

		testEval2.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 2, []*structs.Evaluation{testEval2})

		result2 := <-resultsCh
		must.NoError(t, result2.Err)
	})
	t.Run("stops waiting on constraint failure timeout", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{
			WorkloadTimeout:      50 * time.Millisecond,
			ConcurrentPlacements: 1,
		})
		resultsCh := watcher.Results()

		testEval := mock.Eval()
		testEval.FailedTGAllocs = map[string]*structs.AllocMetric{
			"web": {
				ConstraintFiltered: map[string]int{
					"${attr.kernel.name} == linux": 5,
				},
				NodesExhausted: 0,
			},
		}
		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval})

		ws := memdb.NewWatchSet()
		workload := &testWorkload{eval: testEval.Copy()}
		watcher.WaitForPlacement(t.Context(), workload, ws)

		inProgress := watcher.GetInProgressWorkloads()
		must.Eq(t, 1, len(inProgress))
		must.NotNil(t, inProgress[testEval.ID])

		// Should receive timeout result
		result := <-resultsCh
		must.True(t, result.TimedOut)

		// Should have released the placement slot immediately
		time.Sleep(20 * time.Millisecond)
		must.True(t, watcher.CanAttemptPlacement())

		// Should NOT receive another result (watcher stopped)
		select {
		case <-resultsCh:
			t.Fatal("should not have received second result for constraint failure")
		case <-time.After(100 * time.Millisecond):
			// Expected - no second result
		}
	})

	t.Run("continues waiting on resource exhaustion timeout", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{
			WorkloadTimeout:      50 * time.Millisecond,
			ConcurrentPlacements: 1,
		})
		resultsCh := watcher.Results()

		testEval := mock.Eval()
		testEval.FailedTGAllocs = map[string]*structs.AllocMetric{
			"web": {
				NodesExhausted:     10,
				DimensionExhausted: map[string]int{"cpu": 5},
			},
		}
		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval})

		ws := memdb.NewWatchSet()
		workload := &testWorkload{eval: testEval.Copy()}
		watcher.WaitForPlacement(t.Context(), workload, ws)

		// Should receive timeout result
		result := <-resultsCh
		must.True(t, result.TimedOut)

		// Should still be waiting (placement slot not released yet)
		time.Sleep(20 * time.Millisecond)
		must.False(t, watcher.CanAttemptPlacement())

		inProgress := watcher.GetInProgressWorkloads()
		must.Eq(t, 1, len(inProgress))
		must.NotNil(t, inProgress[testEval.ID])

		// Complete the eval
		testEval.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		// Should receive completion result
		result = <-resultsCh
		must.False(t, result.TimedOut)
		must.NoError(t, result.Err)

		inProgress = watcher.GetInProgressWorkloads()
		must.Eq(t, 0, len(inProgress))

		// Now placement slot should be released
		time.Sleep(20 * time.Millisecond)
		must.True(t, watcher.CanAttemptPlacement())
	})

	t.Run("strict_constraint does not release placement when blocked on a constraint", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{
			WorkloadTimeout:      50 * time.Millisecond,
			ConcurrentPlacements: 1,
			StrictConstraints:    true,
		})
		resultsCh := watcher.Results()

		testEval := mock.Eval()
		testEval.FailedTGAllocs = map[string]*structs.AllocMetric{
			"web": {
				ConstraintFiltered: map[string]int{
					"${attr.kernel.name} == linux": 5,
				},
				NodesExhausted: 0,
			},
		}
		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval})

		ws := memdb.NewWatchSet()
		workload := &testWorkload{eval: testEval.Copy()}
		watcher.WaitForPlacement(t.Context(), workload, ws)

		inProgress := watcher.GetInProgressWorkloads()
		must.Eq(t, 1, len(inProgress))
		must.NotNil(t, inProgress[testEval.ID])

		// Should receive timeout result
		result := <-resultsCh
		must.True(t, result.TimedOut)
		must.Eq(t, result.Workload.GetStatus(), "constrained ${attr.kernel.name} == linux")

		// Should not release the placement slot
		time.Sleep(20 * time.Millisecond)
		must.False(t, watcher.CanAttemptPlacement())

		// Complete the eval
		testEval.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		// Should receive completion result
		result = <-resultsCh
		must.False(t, result.TimedOut)
		must.NoError(t, result.Err)
		time.Sleep(20 * time.Millisecond)
		must.True(t, watcher.CanAttemptPlacement())
	})

	t.Run("drop_workloads does not continue to track workloads", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{
			WorkloadTimeout:      50 * time.Millisecond,
			ConcurrentPlacements: 1,
			DropWorkloads:        true,
		})
		resultsCh := watcher.Results()

		testEval := mock.Eval()
		testEval.FailedTGAllocs = map[string]*structs.AllocMetric{
			"web": {
				ConstraintFiltered: map[string]int{
					"${attr.kernel.name} == linux": 5,
				},
				NodesExhausted: 0,
			},
		}
		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval})

		ws := memdb.NewWatchSet()
		workload := &testWorkload{eval: testEval.Copy()}
		watcher.WaitForPlacement(t.Context(), workload, ws)

		inProgress := watcher.GetInProgressWorkloads()
		must.Eq(t, 1, len(inProgress))
		must.NotNil(t, inProgress[testEval.ID])

		// Should receive timeout result
		result := <-resultsCh
		must.True(t, result.TimedOut)
		must.Eq(t, result.Workload.GetStatus(), "constrained ${attr.kernel.name} == linux")
		must.EqError(t, result.Err, "workload dropped due to constraints")

		// Should release the placement slot
		time.Sleep(20 * time.Millisecond)
		must.True(t, watcher.CanAttemptPlacement())

		inProgress = watcher.GetInProgressWorkloads()
		must.Eq(t, 0, len(inProgress))
	})
}

func TestWorkloadWatcher_isSchedulingComplete(t *testing.T) {
	t.Run("pending eval results in false", func(t *testing.T) {
		ss := state.TestStateStore(t)

		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})
		testEval := mock.Eval()
		testEval.Status = structs.EvalStatusPending
		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval})

		workload := &testWorkload{
			eval: testEval.Copy(),
		}

		complete, err := watcher.IsSchedulingComplete(workload)
		must.NoError(t, err)
		must.False(t, complete)
	})

	t.Run("eval with pending blockedEval results in false", func(t *testing.T) {
		ss := state.TestStateStore(t)

		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})
		testEval := mock.Eval()
		blocked := mock.Eval()

		testEval.Status = structs.EvalStatusComplete
		testEval.BlockedEval = blocked.ID
		blocked.Status = structs.EvalStatusPending

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval, blocked})

		workload := &testWorkload{
			eval: testEval.Copy(),
		}

		complete, err := watcher.IsSchedulingComplete(workload)
		must.NoError(t, err)
		must.False(t, complete)
	})

	t.Run("eval with complete blockedEval results in true", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})

		testEval := mock.Eval()
		blocked := mock.Eval()

		testEval.Status = structs.EvalStatusComplete
		testEval.BlockedEval = blocked.ID
		blocked.Status = structs.EvalStatusComplete

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval, blocked})

		workload := &testWorkload{
			eval: testEval.Copy(),
		}

		complete, err := watcher.IsSchedulingComplete(workload)
		must.NoError(t, err)
		must.True(t, complete)
	})
}

func TestWorkloadWatcher_isConstraintFailure(t *testing.T) {
	t.Run("detects constraint failure without resource exhaustion", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})

		testEval := mock.Eval()
		testEval.FailedTGAllocs = map[string]*structs.AllocMetric{
			"web": {
				ConstraintFiltered: map[string]int{
					"${attr.kernel.name} == linux": 5,
				},
				NodesExhausted:     0,
				ClassExhausted:     map[string]int{},
				DimensionExhausted: map[string]int{},
				QuotaExhausted:     []string{},
			},
		}

		workload := &testWorkload{eval: testEval}
		must.True(t, watcher.isConstraintFailure(workload))
	})

	t.Run("does not detect constraint failure with resource exhaustion", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})

		testEval := mock.Eval()
		testEval.FailedTGAllocs = map[string]*structs.AllocMetric{
			"web": {
				ConstraintFiltered: map[string]int{
					"${attr.kernel.name} == linux": 5,
				},
				NodesExhausted: 10,
				ClassExhausted: map[string]int{"compute": 5},
			},
		}

		workload := &testWorkload{eval: testEval}
		must.False(t, watcher.isConstraintFailure(workload))
	})

	t.Run("detects pure resource exhaustion as not constraint failure", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})

		testEval := mock.Eval()
		testEval.FailedTGAllocs = map[string]*structs.AllocMetric{
			"web": {
				ConstraintFiltered: map[string]int{},
				NodesExhausted:     15,
				DimensionExhausted: map[string]int{"cpu": 10, "memory": 5},
			},
		}

		workload := &testWorkload{eval: testEval}
		must.False(t, watcher.isConstraintFailure(workload))
	})

	t.Run("handles nil FailedTGAllocs", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})

		testEval := mock.Eval()
		testEval.FailedTGAllocs = nil

		workload := &testWorkload{eval: testEval}
		must.False(t, watcher.isConstraintFailure(workload))
	})

	t.Run("handles quota exhaustion as resource issue", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})

		testEval := mock.Eval()
		testEval.FailedTGAllocs = map[string]*structs.AllocMetric{
			"web": {
				ConstraintFiltered: map[string]int{
					"${attr.kernel.name} == linux": 5,
				},
				QuotaExhausted: []string{"default"},
			},
		}

		workload := &testWorkload{eval: testEval}
		must.False(t, watcher.isConstraintFailure(workload))
	})
}

func TestWorkloadWatcher_TrackPlacement(t *testing.T) {
	t.Run("tracks and untracks workloads", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})

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
