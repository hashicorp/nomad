// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"context"
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

	// results channel for communicating placement completions to queues
	results chan PlacementResult

	// Concurrency control
	mu                      sync.Mutex
	workloadTimeout         time.Duration
	currentPlacements       int
	maxConcurrentPlacements int
}

func NewWorkloadWatcher(s Snapshotter, logger hclog.Logger, config *structs.BatchQueue) *WorkloadWatcher {
	w := &WorkloadWatcher{
		stateStore: s,
		config:     config,
		logger:     logger.Named("workload_watcher"),
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

// WaitForPlacement watches an evaluation until it reaches a terminal state or times out.
// It runs async and sends the result to the Results() channel.
func (w *WorkloadWatcher) WaitForPlacement(ctx context.Context, workload Workload, ws memdb.WatchSet) {
	// Track this placement
	w.mu.Lock()
	w.currentPlacements++
	w.mu.Unlock()

	go func() {
		err := w.waitWithTimeout(ctx, workload, ws)

		if err == context.DeadlineExceeded {
			// Send result of the timeout
			w.results <- PlacementResult{
				Workload: workload,
				TimedOut: true,
			}

			// Continue waiting for placement to complete
			err = w.wait(ctx, workload, ws)
		}

		// Release this placement slot
		w.mu.Lock()
		w.currentPlacements--
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
