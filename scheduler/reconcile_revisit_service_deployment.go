package scheduler

import (
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// reconcileServiceDeployment runs the reconciler logic for service jobs when
// there's an active deployment
func reconcileServiceDeployment(eval *structs.Evaluation, job *structs.Job, tg *structs.TaskGroup, d *structs.Deployment, allocs []*structs.Allocation, nodes []*structs.Node) Plan {

	// find the fixed size of our heaps
	previousVersionCounts, currentVersionCounts := statesFromAllocs(eval, job.Version, allocs)
	expectPrevious, expectCurrent := determineExpectedCounts(
		eval,
		tg,
		d,
		previousVersionCounts,
		currentVersionCounts)

	previousAllocs := NewCandidateHeap(expectPrevious)
	currentAllocs := NewCandidateHeap(expectCurrent)
	discardAllocs := NewCandidateHeap(len(allocs))

	plan := Plan{
		previous: previousAllocs,
		current:  currentAllocs,
		discard:  discardAllocs,
		evals:    []*structs.Evaluation{},
	}

	for _, alloc := range allocs {
		// TODO: do we need a different index function for deployments?
		candidate := NewCandidate(alloc, job, tg, genericIndexFuncNoDeploy(job.Version))
		if candidate.JobVersion == job.Version {
			rejected, ok := currentAllocs.Push(candidate)
			if !ok {
				discardAllocs.Push(rejected)
			}
		} else {
			rejected, ok := previousAllocs.Push(candidate)
			if !ok {
				discardAllocs.Push(rejected)
			}
		}
	}

	// make replacements
	for i := 0; i <= currentAllocs.Len(); i++ {
		c, ok := currentAllocs.PeekN(i)
		if !ok {
			break
		}
		if c.Alloc == nil {
			continue // skip over new placements
		}

		switch c.Status {

		case CandidateStatusClientTerminal:
			currentAllocs.Remove(c)
			discardAllocs.Push(c)

		case CandidateStatusDestructiveUpdate, CandidateStatusOnDrainingNode:

			currentAllocs.Remove(c)
			discardAllocs.Push(c)

			candidate := NewCandidateReplacement(c.Alloc, eval, job.Version, "", c.Index)
			currentAllocs.Push(candidate)

		case CandidateStatusNeedsRescheduleNow:

			currentAllocs.Remove(c)
			discardAllocs.Push(c)

			candidate := NewCandidateReplacement(c.Alloc, eval, job.Version, "", c.Index)
			candidate.Alloc.RescheduleTracker.Events = append(
				candidate.Alloc.RescheduleTracker.Events, &structs.RescheduleEvent{
					RescheduleTime: time.Now().Unix(),
					PrevAllocID:    c.Alloc.ID,
					PrevNodeID:     c.Alloc.NodeID,
					Delay:          0,
				})
			currentAllocs.Push(candidate)

		case CandidateStatusNeedsRescheduleLater:
			plan.evals = append(plan.evals, &structs.Evaluation{})

		case CandidateStatusAwaitingReconnect:

		}
	}

	// fill any remaining gap with new placements
	gap := expectCurrent - currentAllocs.Len()
	for i := 0; i < gap; i++ {
		currentAllocs.Push(
			NewCandidate(nil, job, tg, genericIndexFuncNoDeploy(job.Version)))
	}

	return plan
}
