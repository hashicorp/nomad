// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"context"
	"fmt"
	"maps"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
)

type WorkloadWatcher struct {
	stateStore Snapshotter
	logger     hclog.Logger
	mu         sync.Mutex

	inProgressWorkloads map[string]Workload
}

func NewWorkloadWatcher(s Snapshotter, logger hclog.Logger) *WorkloadWatcher {
	w := &WorkloadWatcher{
		stateStore:          s,
		logger:              logger.Named("workload_watcher"),
		inProgressWorkloads: make(map[string]Workload),
	}

	return w
}

// TrackPlacement increments the currentPlacements counter, sets the workload
// status, and adds a workload to the in-progress tracking map.
func (w *WorkloadWatcher) TrackPlacement(workload Workload) {
	workload.SetStatus(WorkloadStatusPlacing, "")
	w.inProgressWorkloads[workload.Eval().ID] = workload
}

// UntrackPlacement decrements the currentPlacements counter and removes a workload from the in-progress tracking map.
func (w *WorkloadWatcher) UntrackPlacement(workload Workload) {
	eval := workload.Eval()

	// If the eval was blocked and then unblocked, the eval will not match in
	// the map. Attempt to use the PreviousEval in that case.
	id := eval.ID
	_, ok := w.inProgressWorkloads[id]
	if !ok {
		id = eval.PreviousEval
		if id == "" {
			return
		}
	}
	delete(w.inProgressWorkloads, id)
}

// GetInProgressWorkloads returns a copy of all workloads currently being watched for placement.
func (w *WorkloadWatcher) GetInProgressWorkloads() map[string]Workload {
	w.mu.Lock()
	defer w.mu.Unlock()

	return maps.Clone(w.inProgressWorkloads)
}

// WaitForPlacement watches an evaluation until it reaches a terminal state or times out.
// It runs async and sends the result to the Results() channel.
func (w *WorkloadWatcher) WaitForPlacement(ctx context.Context, workload Workload) error {
	// Track this placement
	w.mu.Lock()
	w.TrackPlacement(workload)
	w.mu.Unlock()

	err := w.wait(ctx, workload)

	// Remove the workload from tracking
	w.mu.Lock()
	w.UntrackPlacement(workload)
	w.mu.Unlock()

	return err
}

// wait blocks until the workload's evaluation reaches a terminal state.
func (w *WorkloadWatcher) wait(ctx context.Context, workload Workload) error {
	ws := memdb.NewWatchSet()

	for {
		snap, err := w.stateStore.Snapshot()
		if err != nil {
			return err
		}

		job, err := snap.JobByID(ws, workload.Eval().Namespace, workload.Eval().JobID)
		if err != nil {
			return err
		}
		if job == nil {
			w.logger.Debug("watched queue job no longer present in state")
			return nil
		}

		if job.Placed {
			return nil
		}

		if err = ws.WatchCtx(ctx); err != nil {
			return ctx.Err()
		}

		for k := range ws {
			delete(ws, k)
		}
	}
}

// isConstraintFailure checks if the evaluation failed due to non-resource constraints
// (e.g., constraint filters) rather than resource exhaustion.
// Returns true if the failure is constraint-related (and unlikely to resolve with time)
func (w *WorkloadWatcher) isConstraintFailure(workload Workload) (bool, string) {
	eval := workload.Eval()
	if eval == nil || eval.FailedTGAllocs == nil {
		return false, ""
	}

	for _, metric := range eval.FailedTGAllocs {
		if metric == nil {
			continue
		}

		if len(metric.NodesAvailable) == 0 {
			return true, "no nodes available"
		}
		// If there are constraint filters but no resource exhaustion, it's a constraint failure
		hasConstraintFilters := len(metric.ConstraintFiltered) > 0
		hasResourceExhaustion := metric.NodesExhausted > 0 ||
			len(metric.ClassExhausted) > 0 ||
			len(metric.DimensionExhausted) > 0 ||
			len(metric.QuotaExhausted) > 0

		if hasConstraintFilters && !hasResourceExhaustion {
			w.logger.Debug("constraint failure",
				"eval_id", eval.ID,
				"constraint_filtered", metric.ConstraintFiltered)

			reason := ""
			for constraint := range metric.ConstraintFiltered {
				if reason == "" {
					reason = constraint
				} else {
					reason = fmt.Sprintf("%s, %s", reason, constraint)
				}
			}

			return true, reason
		}

	}

	return false, ""
}

// IsSchedulingComplete detects whether a workload was actually placed by following the
// evaluation's BlockedEvals and NextEvals.
// Similar to WaitForPlacement, IsSchedulingComplete will record usage in the event an
// actual placement occurred.
// func (w *WorkloadWatcher) IsSchedulingComplete(workload Workload) (bool, error) {
// 	snap, err := w.stateStore.Snapshot()
// 	if err != nil {
// 		return false, err
// 	}
//
// 	ws := memdb.NewWatchSet()
//
//
// 		eval, err = snap.EvalByID(ws, id)
// 		if err != nil {
// 			return false, err
// 		}
// 		if eval == nil {
// 			return false, ErrWatchedEvalNotFound
// 		}
//
// 		workload.SetEval(eval)
//
// 		if !eval.TerminalStatus() {
// 			return false, nil
// 		}
// 	}
//
// 	if eval.Status == structs.EvalStatusComplete {
// 		return true, nil
// 	}
//
// 	// This would only happen if an eval was not complete and did not
// 	// yet have a followup eval
// 	return false, nil
// }
