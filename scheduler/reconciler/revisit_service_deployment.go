package reconciler

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

func (a *AllocReconciler) computeGroupRevisited(group string, all allocSet) (*ReconcileResults, bool) {

	// Create the output result object that we'll be continuously writing to
	result := new(ReconcileResults)
	result.DesiredTGUpdates = make(map[string]*structs.DesiredUpdates)
	result.DesiredTGUpdates[group] = new(structs.DesiredUpdates)

	// Get the task group. The task group may be nil if the job was updates such
	// that the task group no longer exists
	tg := a.jobState.Job.LookupTaskGroup(group)
	job := a.jobState.Job
	version := job.Version
	evalID := a.jobState.EvalID

	candidates := NewCandidateHeap(tg.Count)
	rejects := NewCandidateHeap(len(all))

	// initial indexing of all the live allocations so we can determine which
	// set of allocs we want to try to keep and/or replace (candidates heap) vs
	// those we know we want to stop (rejects heap)
	for _, alloc := range all {

		var node *structs.Node
		taintedNode, ok := a.clusterState.TaintedNodes[alloc.NodeID]
		if ok {
			node = taintedNode
		}

		candidate := NewCandidate(alloc, node, job, tg,
			genericIndexFuncNoDeploy(tg, version, a.clusterState.Now))
		rejected, ok := candidates.Push(*candidate)
		if !ok {
			rejects.Push(rejected)
		}

	}

	// if we're short on allocs after the initial indexing, first try to
	// populate that with any replacements of rejects before we push brand new
	// placements
	if candidates.Len() < tg.Count {
		for reject := range rejects.Iter() {
			replacement := NewReplacementCandidate(reject.Alloc, evalID, version)
			_, stop := candidates.Push(replacement)
			if stop { // TODO: can we ever get here?
				break
			}
			if candidates.Len() >= tg.Count {
				break
			}
		}

		shortfall := candidates.Len() - tg.Count
		for range shortfall {
			candidate := NewCandidate(nil, nil, job, tg,
				genericIndexFuncNoDeploy(tg, version, a.clusterState.Now))
			candidates.Push(*candidate)
		}

	}

	// turn our list of candidates into results
	for candidate := range candidates.Iter() {
		switch candidate.Status {
		case CandidateStatusDestructiveUpdate:
			//result.DestructiveUpdate = append(result.DestructiveUpdate, candidate.Alloc)
		case CandidateStatusIgnore:
			result.DesiredTGUpdates[group].Ignore++

		case CandidateStatusNeedsRescheduleNow:

			// candidate.Alloc.RescheduleTracker.Events = append(
			// 	candidate.Alloc.RescheduleTracker.Events, &structs.RescheduleEvent{
			// 		RescheduleTime: time.Now().Unix(),
			// 		PrevAllocID:    c.Alloc.ID,
			// 		PrevNodeID:     c.Alloc.NodeID,
			// 		Delay:          0,
			// 	})
		}
	}

	for reject := range rejects.Iter() {
		result.Stop = append(result.Stop, AllocStopResult{
			Alloc:             reject.Alloc,
			ClientStatus:      reject.Alloc.ClientStatus,
			StatusDescription: "", // TODO????
			FollowupEvalID:    "", // TODO???
		})
		result.DesiredTGUpdates[group].Stop++
	}

	return nil, false
}
