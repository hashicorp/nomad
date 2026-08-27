// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

type WorkloadWatcher struct {
	stateStore Snapshotter
	config     *structs.BatchQueue
	logger     hclog.Logger
	mu         sync.Mutex

	// results channel for communicating placement completions to queues
	results chan PlacementResult

	// concurrency control
	workloadTimeout         time.Duration
	maxConcurrentPlacements int

	// placement tracking
	currentPlacements         int
	inProgressWorkloads       map[string]Workload
	strictConstraints         bool
	dropConstrainedPlacements bool
}

func NewWorkloadWatcher(s Snapshotter, logger hclog.Logger, config *structs.BatchQueue) *WorkloadWatcher {
	w := &WorkloadWatcher{
		stateStore:                s,
		config:                    config,
		logger:                    logger.Named("workload_watcher"),
		inProgressWorkloads:       make(map[string]Workload),
		strictConstraints:         config.StrictConstraints,
		dropConstrainedPlacements: config.DropWorkloads,
	}

	if config.WorkloadTimeout != 0 {
		w.workloadTimeout = config.WorkloadTimeout
		w.results = make(chan PlacementResult, config.ConcurrentPlacements)
		w.maxConcurrentPlacements = config.ConcurrentPlacements

	} else {
		w.results = make(chan PlacementResult, 1)
		w.maxConcurrentPlacements = 1
	}

	return w
}

type PlacementResult struct {
	Workload Workload
	Err      error
	TimedOut bool
}

// CanAttemptPlacement returns true if there is capacity for another concurrent placement.
func (w *WorkloadWatcher) CanAttemptPlacement() bool {
	if w.workloadTimeout == 0 {
		return true
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	canAttemptPlacement := w.currentPlacements < w.maxConcurrentPlacements

	if !canAttemptPlacement {
		w.logger.Error("queue is at max concurrent placements")
	}
	return canAttemptPlacement
}

// Results returns the channel that receives placement completion results.
func (w *WorkloadWatcher) Results() <-chan PlacementResult {
	return w.results
}

// TrackPlacement increments the currentPlacements counter, sets the workload
// status, and adds a workload to the in-progress tracking map.
func (w *WorkloadWatcher) TrackPlacement(workload Workload) {
	w.currentPlacements++
	workload.SetStatus("pending", "")
	w.inProgressWorkloads[workload.GetEval().ID] = workload
}

// UntrackPlacement decrements the currentPlacements counter and removes a workload from the in-progress tracking map.
func (w *WorkloadWatcher) UntrackPlacement(workload Workload) {
	eval := workload.GetEval()

	// When strict_constraints is not set, the currentPlacements was already
	// decremented when the workload was marked as constrained. If the watcher
	// is set to drop placements, we want the workload dropped when its
	// constrained.
	if !strings.Contains(workload.GetStatus(), "constrained") {
		w.currentPlacements--
	} else if w.strictConstraints || w.dropConstrainedPlacements {
		w.currentPlacements--
	}

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
func (w *WorkloadWatcher) WaitForPlacement(ctx context.Context, workload Workload, ws memdb.WatchSet) {
	// Track this placement
	w.mu.Lock()
	w.TrackPlacement(workload)
	w.mu.Unlock()

	go func() {

		err := w.waitWithTimeout(ctx, workload, ws)

		if err == context.DeadlineExceeded {
			// Check if timeout was due to non-resource constraints
			if w.isConstraintFailure(workload) {
				w.mu.Lock()

				workload.SetStatus("constrained", w.ConstraintDescription(workload))

				if w.dropConstrainedPlacements {
					w.results <- PlacementResult{
						Workload: workload,
						TimedOut: true,
						Err:      fmt.Errorf("workload dropped due to constraints"),
					}

					w.UntrackPlacement(workload)
					w.mu.Unlock()
					return
				}

				// only release a placement spot if strict constraints are not required
				if !w.strictConstraints {
					w.currentPlacements--
				}

				w.mu.Unlock()
			} else {
				workload.SetStatus("blocked", "timed out")
			}

			// Send result of the timeout
			w.results <- PlacementResult{
				Workload: workload,
				TimedOut: true,
			}

			// Continue waiting for placement to complete. Resource exhaustion
			// may resolve, or stopping the constrained placement will remove it
			// from tracking
			err = w.wait(ctx, workload, ws)
		}

		// Release this placement slot and remove the workload from tracking
		w.mu.Lock()
		w.UntrackPlacement(workload)
		w.mu.Unlock()

		// Send result
		w.results <- PlacementResult{
			Workload: workload,
			Err:      err,
		}
	}()
}

func (w *WorkloadWatcher) waitWithTimeout(ctx context.Context, workload Workload, ws memdb.WatchSet) error {
	// Apply timeout if configured
	if w.config.WorkloadTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.workloadTimeout)
		defer cancel()
	}

	// Wait for placement to complete or timeout
	return w.wait(ctx, workload, ws)

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
