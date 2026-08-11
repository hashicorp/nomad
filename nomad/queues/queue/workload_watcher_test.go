// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"context"
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
}

func (w *testWorkload) GetEval() *structs.Evaluation {
	return w.eval
}
func (w *testWorkload) SetEval(e *structs.Evaluation) {
	w.eval = e
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

		testEval.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		result := <-resultsCh

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

	t.Run("returns deadline exceeded", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{
			WorkloadTimeout:      1 * time.Second,
			ConcurrentPlacements: 1,
		})
		resultsCh := watcher.Results()

		testEval := mock.Eval()
		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval})

		ws := memdb.NewWatchSet()
		workload := &testWorkload{eval: testEval.Copy()}
		watcher.WaitForPlacement(t.Context(), workload, ws)

		result := <-resultsCh

		must.True(t, result.TimedOut)
		must.EqError(t, result.Err, context.DeadlineExceeded.Error())
	})

	t.Run("tracks concurrent placements", func(t *testing.T) {
		ss := state.TestStateStore(t)
		watcher := NewWorkloadWatcher(ss, hclog.Default(), &structs.BatchQueue{
			WorkloadTimeout:      5 * time.Second,
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
