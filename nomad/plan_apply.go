package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
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

		// Evaluate the plan
		result, err := s.evaluatePlan(pending.plan)
		if err != nil {
			s.logger.Printf("[ERR] nomad: failed to evaluate plan: %v", err)
			pending.respond(nil, err)
			continue
		}

		// Apply the plan if there is anything to do
		if len(result.NodeEvict) != 0 || len(result.NodeAllocation) != 0 {
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

// evaluatePlan is used to determine what portions of a plan
// can be applied if any. Returns if there should be a plan application
// which may be partial or if there was an error
func (s *Server) evaluatePlan(plan *structs.Plan) (*structs.PlanResult, error) {
	defer metrics.MeasureSince([]string{"nomad", "plan", "evaluate"}, time.Now())
	// Snapshot the state so that we have a consistent view of the world
	snap, err := s.fsm.State().Snapshot()
	if err != nil {
		return nil, fmt.Errorf("failed to snapshot state: %v", err)
	}

	// Create a result holder for the plan
	result := &structs.PlanResult{
		NodeEvict:      make(map[string][]string),
		NodeAllocation: make(map[string][]*structs.Allocation),
	}

	// Check each allocation to see if it should be allowed
	for nodeID, allocList := range plan.NodeAllocation {
		// Get the node itself
		node, err := snap.GetNodeByID(nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to get node '%s': %v", node, err)
		}

		// Get the existing allocations
		existingAlloc, err := snap.AllocsByNode(nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing allocations for '%s': %v", node, err)
		}

		// Determine the proposed allocation by first removing allocations
		// that are planned evictions and adding the new allocations.
		proposed := existingAlloc
		evictions := plan.NodeEvict[nodeID]
		if len(evictions) > 0 {
			proposed = structs.RemoveAllocs(existingAlloc, evictions)
		}
		proposed = append(proposed, allocList...)

		// Determine if everything fits
		if !AllocationsFit(node, proposed) {
			// Scheduler must have stale data, RefreshIndex should force
			// the latest view of allocations and nodes
			allocIndex, err := snap.GetIndex("allocs")
			if err != nil {
				return nil, err
			}
			nodeIndex, err := snap.GetIndex("node")
			if err != nil {
				return nil, err
			}
			result.RefreshIndex = maxUint64(nodeIndex, allocIndex)

			// If we require all-at-once scheduling, there is no point
			// to continue the evaluation, as we've already failed.
			if plan.AllAtOnce {
				return result, nil
			}

			// Skip this node, since it cannot be used.
			continue
		}

		// Add this to the plan result
		if len(evictions) > 0 {
			result.NodeEvict[nodeID] = evictions
		}
		if len(allocList) > 0 {
			result.NodeAllocation[nodeID] = allocList
		}
	}
	return result, nil
}

// applyPlan is used to apply the plan result and to return the alloc index
func (s *Server) applyPlan(result *structs.PlanResult) (uint64, error) {
	defer metrics.MeasureSince([]string{"nomad", "plan", "apply"}, time.Now())
	req := structs.AllocUpdateRequest{}
	for _, evictList := range result.NodeEvict {
		req.Evict = append(req.Evict, evictList...)
	}
	for _, allocList := range result.NodeAllocation {
		req.Alloc = append(req.Alloc, allocList...)
	}

	_, index, err := s.raftApply(structs.AllocUpdateRequestType, &req)
	return index, err
}

// AllocationsFit checks if a given set of allocations will fit on a node
func AllocationsFit(node *structs.Node, allocs []*structs.Allocation) bool {
	// Start with no resource utilization
	resourcesUsed := new(structs.Resources)

	// Add the reserved resources of the node
	if node.Reserved != nil {
		addResources(resourcesUsed, node.Reserved)
	}

	// For each allocaiton, add the resources
	for _, alloc := range allocs {
		addResources(resourcesUsed, alloc.Resources)
	}

	// Check that the node resources are a super set of those
	// that are being allocated
	if !resourceSubset(node.Resources, resourcesUsed) {
		return false
	}

	// portsOvercommited
	if portsOvercommited(resourcesUsed) {
		return false
	}

	// Everything is in order!
	return true
}

// addResources adds the resources of the delta to the base
func addResources(base, delta *structs.Resources) {
	if base == nil || delta == nil {
		return
	}
	base.CPU += delta.CPU
	base.MemoryMB += delta.MemoryMB
	base.DiskMB += delta.DiskMB
	base.IOPS += delta.IOPS
	for _, net := range delta.Networks {
		if idx := base.NetIndexByCIDR(net.CIDR); idx >= 0 {
			base.Networks[idx].ReservedPorts = append(base.Networks[idx].ReservedPorts,
				net.ReservedPorts...)
			base.Networks[idx].MBits += net.MBits
		}
	}
}

// resourceSubset checks if a resource utilization is a subset of another set
func resourceSubset(super, sub *structs.Resources) bool {
	if super.CPU < sub.CPU {
		return false
	}
	if super.MemoryMB < sub.MemoryMB {
		return false
	}
	if super.DiskMB < sub.DiskMB {
		return false
	}
	if super.IOPS < sub.IOPS {
		return false
	}
	for _, net := range super.Networks {
		idx := sub.NetIndexByCIDR(net.CIDR)
		if idx >= 0 {
			if net.MBits < sub.Networks[idx].MBits {
				return false
			}
		}
	}
	return true
}

// portsOvercommited checks if any of the port resources are over-committed
func portsOvercommited(r *structs.Resources) bool {
	for _, net := range r.Networks {
		ports := make(map[int]struct{})
		for _, port := range net.ReservedPorts {
			if _, ok := ports[port]; ok {
				return true
			}
			ports[port] = struct{}{}
		}
	}
	return false
}
