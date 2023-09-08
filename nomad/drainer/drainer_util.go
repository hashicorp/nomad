// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package drainer

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// defaultMaxIdsPerTxn is the maximum number of IDs that can be included in a
	// single Raft transaction. This is to ensure that the Raft message
	// does not become too large.
	defaultMaxIdsPerTxn = (1024 * 256) / 36 // 0.25 MB of ids.
)

// partitionIds takes a set of IDs and returns a partitioned view of them such
// that no batch would result in an overly large raft transaction.
func partitionIds(maxIds int, ids []string) [][]string {
	index := 0
	total := len(ids)
	var partitions [][]string
	for remaining := total - index; remaining > 0; remaining = total - index {
		if remaining < maxIds {
			partitions = append(partitions, ids[index:])
			break
		} else {
			partitions = append(partitions, ids[index:index+maxIds])
			index += maxIds
		}
	}

	return partitions
}

// transitionTuple is used to group desired transitions and evals
type transitionTuple struct {
	Transitions map[string]*structs.DesiredTransition
	Evals       []*structs.Evaluation
}

// partitionAllocDrain returns a list of alloc transitions and evals to apply
// in a single raft transaction.This is necessary to ensure that the Raft
// transaction does not become too large.
func partitionAllocDrain(maxIds int, transitions map[string]*structs.DesiredTransition,
	evals []*structs.Evaluation) []*transitionTuple {

	// Determine a stable ordering of the transitioning allocs
	allocs := make([]string, 0, len(transitions))
	for id := range transitions {
		allocs = append(allocs, id)
	}

	var requests []*transitionTuple
	submittedEvals, submittedTrans := 0, 0
	for submittedEvals != len(evals) || submittedTrans != len(transitions) {
		req := &transitionTuple{
			Transitions: make(map[string]*structs.DesiredTransition),
		}
		requests = append(requests, req)
		available := maxIds

		// Add the allocs first
		if remaining := len(allocs) - submittedTrans; remaining > 0 {
			if remaining <= available {
				for _, id := range allocs[submittedTrans:] {
					req.Transitions[id] = transitions[id]
				}
				available -= remaining
				submittedTrans += remaining
			} else {
				for _, id := range allocs[submittedTrans : submittedTrans+available] {
					req.Transitions[id] = transitions[id]
				}
				submittedTrans += available

				// Exhausted space so skip adding evals
				continue
			}

		}

		// Add the evals
		if remaining := len(evals) - submittedEvals; remaining > 0 {
			if remaining <= available {
				req.Evals = evals[submittedEvals:]
				submittedEvals += remaining
			} else {
				req.Evals = evals[submittedEvals : submittedEvals+available]
				submittedEvals += available
			}
		}
	}

	return requests
}
