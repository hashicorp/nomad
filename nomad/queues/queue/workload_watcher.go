// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

type WorkloadWatcher struct {
	stateStore Snapshotter
	config     *structs.BatchQueue
	logger     hclog.Logger
	mu         sync.Mutex

	inProgressWorkloads map[string]Workload
}

func NewWorkloadWatcher(s Snapshotter, logger hclog.Logger, config *structs.BatchQueue) *WorkloadWatcher {
	w := &WorkloadWatcher{
		stateStore:          s,
		config:              config,
		logger:              logger.Named("workload_watcher"),
		inProgressWorkloads: make(map[string]Workload),
	}

	return w
}

// TrackPlacement increments the currentPlacements counter, sets the workload
// status, and adds a workload to the in-progress tracking map.
func (w *WorkloadWatcher) TrackPlacement(workload Workload) {
	workload.SetStatus("placing", "")
	w.inProgressWorkloads[workload.GetEval().ID] = workload
}

// UntrackPlacement decrements the currentPlacements counter and removes a workload from the in-progress tracking map.
func (w *WorkloadWatcher) UntrackPlacement(workload Workload) {
	eval := workload.GetEval()

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
	return w.inProgressWorkloads
}

// WaitForPlacement watches an evaluation until it reaches a terminal state or times out.
// It runs async and sends the result to the Results() channel.
func (w *WorkloadWatcher) WaitForPlacement(ctx context.Context, workload Workload, ws memdb.WatchSet) error {
	// Track this placement
	w.mu.Lock()
	w.TrackPlacement(workload)
	w.mu.Unlock()

	err := w.wait(ctx, workload, ws)

	// Remove the workload from tracking
	w.mu.Lock()
	w.UntrackPlacement(workload)
	w.mu.Unlock()

	return err
}

// wait blocks until the workload's evaluation reaches a terminal state.
func (w *WorkloadWatcher) wait(ctx context.Context, workload Workload, ws memdb.WatchSet) error {
	eval := workload.GetEval()

	for !eval.TerminalStatus() || eval.BlockedEval != "" || eval.NextEval != "" {
		// Determine which eval to follow
		evalID := eval.ID
		if eval.BlockedEval != "" {
			evalID = eval.BlockedEval
		} else if eval.NextEval != "" {
			evalID = eval.NextEval
		}

		// Get a snapshot of the state
		snap, err := w.stateStore.Snapshot()
		if err != nil {
			return err
		}

		// Watch for snapshot abandonment
		ws.Add(snap.AbandonCh())

		// Lookup the evaluation
		eval, err = snap.EvalByID(ws, evalID)
		if err != nil {
			return err
		}
		if eval == nil {
			return ErrWatchedEvalNotFound
		}

		workload.SetEval(eval)

		// If terminal, continue to check for followup evals
		if eval.TerminalStatus() {
			continue
		}

		if w.isConstraintFailure(workload) {
			workload.SetStatus("constrained", w.ConstraintDescription(workload))
		}

		// Wait for eval update or context cancellation
		if err = ws.WatchCtx(ctx); err != nil {
			return err
		}

		// Clear the watchset for next iteration
		for k := range ws {
			delete(ws, k)
		}
	}
	return nil
}

// isConstraintFailure checks if the evaluation failed due to non-resource constraints
// (e.g., constraint filters) rather than resource exhaustion.
// Returns true if the failure is constraint-related (and unlikely to resolve with time)
func (w *WorkloadWatcher) isConstraintFailure(workload Workload) bool {
	eval := workload.GetEval()
	if eval == nil || eval.FailedTGAllocs == nil {
		return false
	}

	for _, metric := range eval.FailedTGAllocs {
		if metric == nil {
			continue
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
			return true
		}
	}

	return false
}

// ConstraintDescription returns the constraint filtered causing the workload to not be placed.
func (w *WorkloadWatcher) ConstraintDescription(workload Workload) string {
	eval := workload.GetEval()

	var s string
	for _, metric := range eval.FailedTGAllocs {
		if metric == nil {
			continue
		}

		for constraint := range metric.ConstraintFiltered {
			if s != "" {
				s = fmt.Sprintf("%s, %s", s, constraint)
			} else {
				s = constraint
			}
		}
	}

	return s
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
