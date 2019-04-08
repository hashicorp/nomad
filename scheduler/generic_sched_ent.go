// +build pro ent

package scheduler

import "github.com/hashicorp/nomad/nomad/structs"

// selectNextOption calls the stack to get a node for placement. We retry with eviction enabled if no eligible
// nodes are found
func (s *GenericScheduler) selectNextOption(tg *structs.TaskGroup, selectOptions *SelectOptions) *RankedNode {
	option := s.stack.Select(tg, selectOptions)
	// Run stack again with preemption enabled
	if option == nil {
		selectOptions.Preempt = true
		option = s.stack.Select(tg, selectOptions)
	}
	return option
}

// handlePreemptions sets relevant preeemption related fields
func (s *GenericScheduler) handlePreemptions(option *RankedNode, alloc *structs.Allocation, missing placementResult) {
	if option.PreemptedAllocs == nil {
		return
	}

	// If this placement involves preemption, set DesiredState to evict for those allocations
	var preemptedAllocIDs []string
	for _, stop := range option.PreemptedAllocs {
		s.plan.AppendPreemptedAlloc(stop, structs.AllocDesiredStatusEvict, alloc.ID)
		preemptedAllocIDs = append(preemptedAllocIDs, stop.ID)

		if s.eval.AnnotatePlan && s.plan.Annotations != nil {
			s.plan.Annotations.PreemptedAllocs = append(s.plan.Annotations.PreemptedAllocs, stop.Stub())
			if s.plan.Annotations.DesiredTGUpdates != nil {
				desired := s.plan.Annotations.DesiredTGUpdates[missing.TaskGroup().Name]
				desired.Preemptions += 1
			}
		}
	}

	alloc.PreemptedAllocations = preemptedAllocIDs
}
