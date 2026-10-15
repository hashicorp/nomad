// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

type WorkloadWatcher struct {
	logger     hclog.Logger
	stateStore Snapshotter
	mu         sync.Mutex

	timeout time.Duration

	inProgressWorkloads map[string]Workload
}

func NewWorkloadWatcher(logger hclog.Logger, s Snapshotter, timeout time.Duration) *WorkloadWatcher {
	w := &WorkloadWatcher{
		logger:              logger.Named("workload_watcher"),
		stateStore:          s,
		timeout:             timeout,
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

		eval := workload.Eval()

		job, err := snap.JobByID(ws, eval.Namespace, eval.JobID)
		if err != nil {
			return err
		}
		if job == nil {
			w.logger.Debug("watched queue job no longer present in state")
			return nil
		}

		if job.Placed || job.Stopped() {
			return nil
		}

		allEvals, err := snap.EvalsByJob(ws, eval.Namespace, eval.JobID)
		if err != nil {
			return err
		}

		watchCtx, cancel := ctx, context.CancelFunc(func() {})
		for _, e := range allEvals {
			if e.Status == structs.EvalStatusBlocked && w.isConstraintFailure(e) {
				watchCtx, cancel = context.WithDeadline(ctx, time.Unix(0, e.CreateTime).Add(w.timeout))
			}
		}

		err = ws.WatchCtx(watchCtx)
		cancel()

		if err != nil {
			w.logger.Debug("shutting down workload watcher for job", "ID", workload.ID(), "err", watchCtx.Err())
			return nil
		}

		for k := range ws {
			delete(ws, k)
		}
	}
}

// isConstraintFailure checks if the evaluation failed due to non-resource constraints
// (e.g., constraint filters) rather than resource exhaustion.
// Returns true if the failure is constraint-related (and unlikely to resolve with time)
func (w *WorkloadWatcher) isConstraintFailure(eval *structs.Evaluation) bool {
	if eval == nil || eval.FailedTGAllocs == nil {
		return false
	}

	for _, metric := range eval.FailedTGAllocs {
		if metric == nil {
			continue
		}

		if len(metric.NodesAvailable) == 0 {
			return true
		}
		// If there are constraint filters but no resource exhaustion, it's a constraint failure
		hasConstraintFilters := len(metric.ConstraintFiltered) > 0
		hasResourceExhaustion := metric.NodesExhausted > 0 ||
			len(metric.ClassExhausted) > 0 ||
			len(metric.DimensionExhausted) > 0 ||
			len(metric.QuotaExhausted) > 0

		if hasConstraintFilters && !hasResourceExhaustion {
			reason := ""
			for constraint := range metric.ConstraintFiltered {
				if reason == "" {
					reason = constraint
				} else {
					reason = fmt.Sprintf("%s, %s", reason, constraint)
				}
			}
			w.logger.Debug("constraint failure",
				"eval_id", eval.ID,
				"constraint_filtered", reason)

			return true
		}

	}

	return false
}
