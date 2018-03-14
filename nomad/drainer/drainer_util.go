package drainer

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// maxIdsPerTxn is the maximum number of IDs that can be included in a
	// single Raft transaction. This is to ensure that the Raft message does not
	// become too large.
	maxIdsPerTxn = (1024 * 256) / 36 // 0.25 MB of ids.
)

// partitionIds takes a set of IDs and returns a partitioned view of them such
// that no batch would result in an overly large raft transaction.
func partitionIds(ids []string) [][]string {
	index := 0
	total := len(ids)
	var partitions [][]string
	for remaining := total - index; remaining > 0; remaining = total - index {
		if remaining < maxIdsPerTxn {
			partitions = append(partitions, ids[index:])
			break
		} else {
			partitions = append(partitions, ids[index:index+maxIdsPerTxn])
			index += maxIdsPerTxn
		}
	}

	return partitions
}

// transistionTuple is used to group desired transistions and evals
type transistionTuple struct {
	Transistions map[string]*structs.DesiredTransition
	Evals        []*structs.Evaluation
}

// partitionAllocDrain returns a list of alloc transistions and evals to apply
// in a single raft transaction.This is necessary to ensure that the Raft
// transaction does not become too large.
func partitionAllocDrain(transistions map[string]*structs.DesiredTransition,
	evals []*structs.Evaluation) []*transistionTuple {

	// Determine a stable ordering of the transistioning allocs
	allocs := make([]string, 0, len(transistions))
	for id := range transistions {
		allocs = append(allocs, id)
	}

	var requests []*transistionTuple
	submittedEvals, submittedTrans := 0, 0
	for submittedEvals != len(evals) || submittedTrans != len(transistions) {
		req := &transistionTuple{
			Transistions: make(map[string]*structs.DesiredTransition),
		}
		requests = append(requests, req)
		available := maxIdsPerTxn

		// Add the allocs first
		if remaining := len(allocs) - submittedTrans; remaining > 0 {
			if remaining <= available {
				for _, id := range allocs[submittedTrans:] {
					req.Transistions[id] = transistions[id]
				}
				available -= remaining
				submittedTrans += remaining
			} else {
				for _, id := range allocs[submittedTrans : submittedTrans+available] {
					req.Transistions[id] = transistions[id]
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
