package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/Godeps/_workspace/src/github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// planApply is a long lived goroutine that reads plan allocations from
// the plan queue, determines if they can be applied safely and applies
// them via Raft.
func (s *Server) planApply() {
	for {
		// Pull the next pending plan, exit if we are no longer leader
		pending, err := s.planQueue.Dequeue(0)
		if err != nil {
			return
		}

		// Verify the evaluation is outstanding, and that the tokens match.
		token, ok := s.evalBroker.Outstanding(pending.plan.EvalID)
		if !ok {
			s.logger.Printf("[ERR] nomad: plan received for non-outstanding evaluation %s",
				pending.plan.EvalID)
			pending.respond(nil, fmt.Errorf("evaluation is not outstanding"))
			continue
		}
		if pending.plan.EvalToken != token {
			s.logger.Printf("[ERR] nomad: plan received for evaluation %s with wrong token",
				pending.plan.EvalID)
			pending.respond(nil, fmt.Errorf("evaluation token does not match"))
			continue
		}

		// Snapshot the state so that we have a consistent view of the world
		snap, err := s.fsm.State().Snapshot()
		if err != nil {
			s.logger.Printf("[ERR] nomad: failed to snapshot state: %v", err)
			pending.respond(nil, err)
			continue
		}

		// Evaluate the plan
		result, err := evaluatePlan(snap, pending.plan)
		if err != nil {
			s.logger.Printf("[ERR] nomad: failed to evaluate plan: %v", err)
			pending.respond(nil, err)
			continue
		}

		// Apply the plan if there is anything to do
		if !result.IsNoOp() {
			allocIndex, err := s.applyPlan(result)
			if err != nil {
				s.logger.Printf("[ERR] nomad: failed to apply plan: %v", err)
				pending.respond(nil, err)
				continue
			}
			result.AllocIndex = allocIndex
		}

		// Respond to the plan
		pending.respond(result, nil)
	}
}

// applyPlan is used to apply the plan result and to return the alloc index
func (s *Server) applyPlan(result *structs.PlanResult) (uint64, error) {
	defer metrics.MeasureSince([]string{"nomad", "plan", "apply"}, time.Now())
	req := structs.AllocUpdateRequest{}
	for _, updateList := range result.NodeUpdate {
		req.Alloc = append(req.Alloc, updateList...)
	}
	for _, allocList := range result.NodeAllocation {
		req.Alloc = append(req.Alloc, allocList...)
	}
	req.Alloc = append(req.Alloc, result.FailedAllocs...)

	_, index, err := s.raftApply(structs.AllocUpdateRequestType, &req)
	return index, err
}

// evaluatePlan is used to determine what portions of a plan
// can be applied if any. Returns if there should be a plan application
// which may be partial or if there was an error
func evaluatePlan(snap *state.StateSnapshot, plan *structs.Plan) (*structs.PlanResult, error) {
	defer metrics.MeasureSince([]string{"nomad", "plan", "evaluate"}, time.Now())

	// Create a result holder for the plan
	result := &structs.PlanResult{
		NodeUpdate:     make(map[string][]*structs.Allocation),
		NodeAllocation: make(map[string][]*structs.Allocation),
		FailedAllocs:   plan.FailedAllocs,
	}

	// Collect all the nodeIDs
	nodeIDs := make(map[string]struct{})
	for nodeID := range plan.NodeUpdate {
		nodeIDs[nodeID] = struct{}{}
	}
	for nodeID := range plan.NodeAllocation {
		nodeIDs[nodeID] = struct{}{}
	}

	// Check each allocation to see if it should be allowed
	for nodeID := range nodeIDs {
		// Evaluate the plan for this node
		fit, err := evaluateNodePlan(snap, plan, nodeID)
		if err != nil {
			return nil, err
		}
		if !fit {
			// Scheduler must have stale data, RefreshIndex should force
			// the latest view of allocations and nodes
			allocIndex, err := snap.Index("allocs")
			if err != nil {
				return nil, err
			}
			nodeIndex, err := snap.Index("nodes")
			if err != nil {
				return nil, err
			}
			result.RefreshIndex = maxUint64(nodeIndex, allocIndex)

			// If we require all-at-once scheduling, there is no point
			// to continue the evaluation, as we've already failed.
			if plan.AllAtOnce {
				result.NodeUpdate = nil
				result.NodeAllocation = nil
				return result, nil
			}

			// Skip this node, since it cannot be used.
			continue
		}

		// Add this to the plan result
		if nodeUpdate := plan.NodeUpdate[nodeID]; len(nodeUpdate) > 0 {
			result.NodeUpdate[nodeID] = nodeUpdate
		}
		if nodeAlloc := plan.NodeAllocation[nodeID]; len(nodeAlloc) > 0 {
			result.NodeAllocation[nodeID] = nodeAlloc
		}
	}
	return result, nil
}

// evaluateNodePlan is used to evalute the plan for a single node,
// returning if the plan is valid or if an error is encountered
func evaluateNodePlan(snap *state.StateSnapshot, plan *structs.Plan, nodeID string) (bool, error) {
	// If this is an evict-only plan, it always 'fits' since we are removing things.
	if len(plan.NodeAllocation[nodeID]) == 0 {
		return true, nil
	}

	// Get the node itself
	node, err := snap.NodeByID(nodeID)
	if err != nil {
		return false, fmt.Errorf("failed to get node '%s': %v", node, err)
	}

	// If the node does not exist or is not ready for schduling it is not fit
	// XXX: There is a potential race between when we do this check and when
	// the Raft commit happens.
	if node == nil || node.Status != structs.NodeStatusReady || node.Drain {
		return false, nil
	}

	// Get the existing allocations
	existingAlloc, err := snap.AllocsByNode(nodeID)
	if err != nil {
		return false, fmt.Errorf("failed to get existing allocations for '%s': %v", node, err)
	}

	// Filter on alloc state
	existingAlloc = structs.FilterTerminalAllocs(existingAlloc)

	// Determine the proposed allocation by first removing allocations
	// that are planned evictions and adding the new allocations.
	proposed := existingAlloc
	var remove []*structs.Allocation
	if update := plan.NodeUpdate[nodeID]; len(update) > 0 {
		remove = append(remove, update...)
	}
	if updated := plan.NodeAllocation[nodeID]; len(updated) > 0 {
		for _, alloc := range updated {
			remove = append(remove, alloc)
		}
	}
	proposed = structs.RemoveAllocs(existingAlloc, remove)
	proposed = append(proposed, plan.NodeAllocation[nodeID]...)

	// Check if these allocations fit
	fit, _, _, err := structs.AllocsFit(node, proposed, nil)
	return fit, err
}
