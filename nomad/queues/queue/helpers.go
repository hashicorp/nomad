// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"context"

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

// WaitForPlacement follows a given evaluation in the state store until it, or its next/blocked evals
// have been marked terminal, indicating the workload has been scheduled.
//
// Note: If a job with an unsatisfiable contraint is given to the Eval Broker, this function will block
// until a Nomad operator manually intervenes and stops the job. In the future, we can add an optional
// configurable timeout for this blocking query.
func WaitForPlacement(ctx context.Context, workload Workload, ss *state.StateStore, ws memdb.WatchSet) error {
	eval := workload.GetEval()
	for !eval.TerminalStatus() || eval.BlockedEval != "" || eval.NextEval != "" {
		id := eval.ID

		if eval.BlockedEval != "" {
			id = eval.BlockedEval
		} else if eval.NextEval != "" {
			id = eval.NextEval
		}

		snap, err := ss.Snapshot()
		if err != nil {
			return err
		}

		// TODO: handle snapshot restores
		abandonCh := snap.AbandonCh()
		ws.Add(abandonCh)

		eval, err = snap.EvalByID(ws, id)
		if err != nil {
			return err
		}
		if eval == nil {
			return ErrWatchedEvalNotFound
		}

		workload.SetEval(eval)

		if eval.TerminalStatus() {
			continue
		}

		// If the latest version of the eval isn't terminal, wait for an update
		if err = ws.WatchCtx(ctx); err != nil {
			return err
		}

		// The watch channel will be closed, we should delete it to
		// prevent immediately firing on the next WatchCtx
		for k := range ws {
			delete(ws, k)
		}
	}

	return nil
}

// IsSchedulingComplete detects whether a workload was actually placed by following the
// evaluation's BlockedEvals and NextEvals.
// Similar to WaitForPlacement, IsSchedulingComplete will record usage in the event an
// actual placement occurred.
func IsSchedulingComplete(workload Workload, ss *state.StateStore) (bool, error) {
	snap, err := ss.Snapshot()
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
