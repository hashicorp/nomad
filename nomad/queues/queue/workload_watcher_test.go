// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"fmt"
	"strings"
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

		testEval := mock.Eval()
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		ws := memdb.NewWatchSet()
		workload := &testWorkload{eval: testEval.Copy()}
		doneCh := make(chan error)
		go func() {
			err := watcher.WaitForPlacement(t.Context(), workload, ws)
			doneCh <- err
		}()

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
		case <-doneCh:
			t.Fatal("should not have exited")
		default:
		}

		inProgress := watcher.GetInProgressWorkloads()
		must.Eq(t, 1, len(inProgress))
		must.NotNil(t, inProgress[testEval.ID])
		workingWorkload := inProgress[testEval.ID]
		must.Eq(t, workingWorkload.GetStatus(), "placing ")

		testEval.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		done := <-doneCh
		must.NoError(t, done)

		inProgress = watcher.GetInProgressWorkloads()
		must.Eq(t, 0, len(inProgress))

	})

	t.Run("continues watching blocked evals", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})

		testEval := mock.Eval()
		blocked := mock.Eval()

		testEval.Status = structs.EvalStatusComplete
		testEval.BlockedEval = blocked.ID

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval, blocked})

		ws := memdb.NewWatchSet()
		workload := &testWorkload{eval: testEval.Copy()}
		doneCh := make(chan error)
		go func() {
			err := watcher.WaitForPlacement(t.Context(), workload, ws)
			doneCh <- err
		}()

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
		case <-doneCh:
			t.Fatal("should not have exited")
		default:
		}

		blocked.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{blocked})

		done := <-doneCh
		must.NoError(t, done)
	})

	t.Run("continues watching next evals after eval failure", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})

		testEval := mock.Eval()
		next := mock.Eval()

		testEval.Status = structs.EvalStatusFailed
		testEval.NextEval = next.ID

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval, next})

		ws := memdb.NewWatchSet()
		workload := &testWorkload{eval: testEval.Copy()}
		doneCh := make(chan error)
		go func() {
			err := watcher.WaitForPlacement(t.Context(), workload, ws)
			doneCh <- err
		}()

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
		case <-doneCh:
			t.Fatal("should not have exited")
		default:
		}

		next.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{next})

		done := <-doneCh
		must.NoError(t, done)
	})

	t.Run("updates status when constrained", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})

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
		doneCh := make(chan error)
		go func() {
			err := watcher.WaitForPlacement(t.Context(), workload, ws)
			doneCh <- err
		}()

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
		case <-doneCh:
			t.Fatal("should not have exited")
		default:
		}

		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		// We want to make sure the testQueue has begun a watch on the blocked eval
		// before continuing, which is indicated by the length of the watchset being >0.
		must.Wait(t, wait.InitialSuccess(
			wait.BoolFunc(func() bool {
				return strings.Contains(workload.s, "constrained")
			}),
			wait.Timeout(5*time.Second),
			wait.Gap(100*time.Millisecond),
		))

		inProgress := watcher.GetInProgressWorkloads()
		must.Eq(t, 1, len(inProgress))
		must.NotNil(t, inProgress[testEval.ID])
		workingWorkload := inProgress[testEval.ID]
		must.Eq(t, workingWorkload.GetStatus(), "constrained ${attr.kernel.name} == linux")

		// Complete the eval
		testEval.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		done := <-doneCh
		must.NoError(t, done)

		inProgress = watcher.GetInProgressWorkloads()
		must.Eq(t, 0, len(inProgress))
	})

	t.Run("continues waiting on resource exhaustion timeout", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{})

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
		doneCh := make(chan error)
		go func() {
			err := watcher.WaitForPlacement(t.Context(), workload, ws)
			doneCh <- err
		}()

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

		inProgress := watcher.GetInProgressWorkloads()
		must.Eq(t, 1, len(inProgress))
		must.NotNil(t, inProgress[testEval.ID])

		// Complete the eval
		testEval.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		done := <-doneCh
		must.NoError(t, done)

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
