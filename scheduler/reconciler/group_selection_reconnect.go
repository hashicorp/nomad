// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// PickGroupSelectionCohort applies the disconnect reconciliation policy to
// complete cohorts so reconnection converges on one implementation per slot.
// Score comparisons use mean available scores; longest_running compares
// the earliest main-task start, preserving the native tie and fallback rules.
func PickGroupSelectionCohort(strategy *structs.DisconnectStrategy, original, replacement []*structs.Allocation) bool {
	if len(original) == 0 {
		return false
	}
	if len(replacement) == 0 {
		return true
	}
	for _, alloc := range replacement {
		if alloc.Job.Version > original[0].Job.Version || alloc.Job.CreateIndex > original[0].Job.CreateIndex {
			return false
		}
	}
	switch strategy.ReconcileStrategy() {
	case structs.ReconcileOptionKeepOriginal:
		return true
	case structs.ReconcileOptionKeepReplacement:
		return false
	case structs.ReconcileOptionLongestRunning:
		oldStart, newStart := cohortStart(original), cohortStart(replacement)
		if oldStart.IsZero() && !newStart.IsZero() {
			return false
		}
		if !oldStart.IsZero() && newStart.IsZero() {
			return true
		}
		if !oldStart.IsZero() || !newStart.IsZero() {
			return oldStart.Before(newStart)
		}
	}
	oldScore, oldScored, oldRunning := cohortScore(original)
	newScore, newScored, newRunning := cohortScore(replacement)
	better := (!oldScored && newScored) || (oldScored && newScored && newScore > oldScore)
	return !(better && (newRunning || !oldRunning))
}

func cohortScore(allocations []*structs.Allocation) (mean float64, scored, running bool) {
	count := 0
	for _, alloc := range allocations {
		if alloc.TerminalStatus() {
			continue
		}
		if alloc.ClientStatus == structs.AllocClientStatusRunning {
			running = true
		}
		if score := alloc.Metrics.MaxNormScore(); score != nil {
			mean += score.NormScore
			count++
		}
	}
	if count > 0 {
		mean /= float64(count)
		scored = true
	}
	return
}

func cohortStart(allocations []*structs.Allocation) time.Time {
	var start time.Time
	for _, alloc := range allocations {
		if alloc.TerminalStatus() {
			continue
		}
		current := startOfLeaderOrOldestTaskInMain(alloc, alloc.Job.LookupTaskGroup(alloc.TaskGroup))
		if !current.IsZero() && (start.IsZero() || current.Before(start)) {
			start = current
		}
	}
	return start
}
