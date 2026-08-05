// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

func CmpWaitOnRestore(a, b Workload) int {
	if a.WaitOnRestore() && !b.WaitOnRestore() {
		return -1
	} else if !a.WaitOnRestore() && b.WaitOnRestore() {
		return 1
	}
	return 0
}

type WorkloadWatcher struct {
	stateStore *state.StateStore
	config     *structs.BatchQueue
	logger     hclog.Logger
	setTimeout bool
}

func NewWorkloadWatcher(s *state.StateStore, config *structs.BatchQueue) *WorkloadWatcher {
	w := &WorkloadWatcher{stateStore: s, config: config}
	if config != nil {
		w.setTimeout = true

	}
	return w
}

// WaitForPlacement follows a given evaluation in the state store until it, or its next/blocked evals
// have been marked terminal, indicating the workload has been scheduled.
//
// Note: If a job with an unsatisfiable contraint is given to the Eval Broker, this function will block
// until a Nomad operator manually intervenes and stops the job. In the future, we can add an optional
// configurable timeout for this blocking query.
func (w *WorkloadWatcher) WaitForPlacement(ctx context.Context, workload Workload, ws memdb.WatchSet) error {
	if w.setTimeout {
		timedOut, cancelFn := context.WithTimeout(ctx, w.config.WorkloadTimeout)
		ctx = timedOut
		defer cancelFn()
	}

	err := w.wait(ctx, workload, ws)
	//	if err == context.DeadlineExceeded {
	//	}
	return err
}

func (w *WorkloadWatcher) wait(ctx context.Context, workload Workload, ws memdb.WatchSet) error {
	workCh := make(chan error, 1)
	go func() {
		eval := workload.GetEval()
		for !eval.TerminalStatus() || eval.BlockedEval != "" || eval.NextEval != "" {
			id := eval.ID

			if eval.BlockedEval != "" {
				// if due to constraints, do something
				id = eval.BlockedEval
			} else if eval.NextEval != "" {
				id = eval.NextEval
			}

			snap, err := w.stateStore.Snapshot()
			if err != nil {
				workCh <- err
				return
			}

			// TODO: handle snapshot restores
			abandonCh := snap.AbandonCh()
			ws.Add(abandonCh)

			eval, err = snap.EvalByID(ws, id)
			if err != nil {
				workCh <- err
				return
			}
			if eval == nil {
				workCh <- ErrWatchedEvalNotFound
				return
			}

			workload.SetEval(eval)

			if eval.TerminalStatus() {
				continue
			}

			// If the latest version of the eval isn't terminal, wait for an update
			if err = ws.WatchCtx(ctx); err != nil {
				workCh <- err
				return
			}

			// The watch channel will be closed, we should delete it to
			// prevent immediately firing on the next WatchCtx
			for k := range ws {
				delete(ws, k)
			}
		}
		workCh <- nil
	}()

	select {
	case e := <-workCh:
		return e
	case <-ctx.Done():
		return ctx.Err()
	}

}

// IsSchedulingComplete detects whether a workload was actually placed by following the
// evaluation's BlockedEvals and NextEvals.
// Similar to WaitForPlacement, IsSchedulingComplete will record usage in the event an
// actual placement occurred.
func (w *WorkloadWatcher) IsSchedulingComplete(workload Workload) (bool, error) {
	snap, err := w.stateStore.Snapshot()
	if err != nil {
		return false, err
	}

	ws := memdb.NewWatchSet()
	eval := workload.GetEval()
	for eval.BlockedEval != "" || eval.NextEval != "" {
		id := eval.ID

		if eval.BlockedEval != "" {
			id = eval.BlockedEval
		} else if eval.NextEval != "" {
			id = eval.NextEval
		}

		eval, err = snap.EvalByID(ws, id)
		if err != nil {
			return false, err
		}
		if eval == nil {
			return false, ErrWatchedEvalNotFound
		}

		workload.SetEval(eval)

		if !eval.TerminalStatus() {
			return false, nil
		}
	}

	if eval.Status == structs.EvalStatusComplete {
		return true, nil
	}

	// This would only happen if an eval was not complete and did not
	// yet have a followup eval
	return false, nil
}
