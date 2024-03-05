// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconnectingpicker

import (
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

type ReconnectingPicker struct {
	logger log.Logger
}

func New(logger log.Logger) *ReconnectingPicker {
	rp := ReconnectingPicker{
		logger: log.L().Named("reconnecting-picker"),
	}

	return &rp
}

func (rp *ReconnectingPicker) PickReconnectingAlloc(ds *structs.DisconnectStrategy, original *structs.Allocation, replacement *structs.Allocation) *structs.Allocation {
	// Check if the replacement is newer.
	// Always prefer the replacement if true.
	replacementIsNewer := replacement.Job.Version > original.Job.Version ||
		replacement.Job.CreateIndex > original.Job.CreateIndex
	if replacementIsNewer {
		rp.logger.Debug("replacement has a newer version, keeping replacement")
		return replacement
	}

	var picker func(*structs.Allocation, *structs.Allocation) *structs.Allocation

	rs := ds.ReconcileStrategy()
	rp.logger.Debug("picking according to strategy", "strategy", rs)

	switch rs {
	case structs.ReconcileOptionBestScore:
		picker = rp.pickBestScore

	case structs.ReconcileOptionKeepOriginal:
		picker = rp.pickOriginal

	case structs.ReconcileOptionKeepReplacement:
		picker = rp.pickReplacement

	case structs.ReconcileOptionLongestRunning:
		picker = rp.pickLongestRunning
	}

	return picker(original, replacement)
}

// pickReconnectingAlloc returns the allocation to keep between the original
// one that is reconnecting and one of its replacements.
//
// This function is not commutative, meaning that pickReconnectingAlloc(A, B)
// is not the same as pickReconnectingAlloc(B, A). Preference is given to keep
// the original allocation when possible.
func (rp *ReconnectingPicker) pickBestScore(original *structs.Allocation, replacement *structs.Allocation) *structs.Allocation {

	// Check if the replacement has better placement score.
	// If any of the scores is not available, only pick the replacement if
	// itself does have scores.
	originalMaxScoreMeta := original.Metrics.MaxNormScore()
	replacementMaxScoreMeta := replacement.Metrics.MaxNormScore()

	replacementHasBetterScore := originalMaxScoreMeta == nil && replacementMaxScoreMeta != nil ||
		(originalMaxScoreMeta != nil && replacementMaxScoreMeta != nil &&
			replacementMaxScoreMeta.NormScore > originalMaxScoreMeta.NormScore)

	// Check if the replacement has better client status.
	// Even with a better placement score make sure we don't replace a running
	// allocation with one that is not.
	replacementIsRunning := replacement.ClientStatus == structs.AllocClientStatusRunning
	originalNotRunning := original.ClientStatus != structs.AllocClientStatusRunning

	if replacementHasBetterScore && (replacementIsRunning || originalNotRunning) {
		return replacement
	}

	return original
}

func (rp *ReconnectingPicker) pickOriginal(original *structs.Allocation, _ *structs.Allocation) *structs.Allocation {
	return original
}

func (rp *ReconnectingPicker) pickReplacement(_ *structs.Allocation, replacement *structs.Allocation) *structs.Allocation {
	return replacement
}

func (rp *ReconnectingPicker) pickLongestRunning(original *structs.Allocation, replacement *structs.Allocation) *structs.Allocation {
	ln := original.GetLeaderTasksName()

	// Default to the first task in the group if no leader is found.
	if ln == "" {
		tg := original.Job.LookupTaskGroup(original.TaskGroup)
		ln = tg.Tasks[0].Name
	}

	if original.LatestStartOfTask(ln).Sub(replacement.LatestStartOfTask(ln)) < 0 {
		return original
	}

	return replacement
}
