// +build pro ent

package scheduler

import "github.com/hashicorp/nomad/nomad/structs"

// selectNextOption calls the stack to get a node for placement
func (s *GenericScheduler) selectNextOption(tg *structs.TaskGroup, selectOptions *SelectOptions) *RankedNode {
	option := s.stack.Select(tg, selectOptions)
	_, schedConfig, _ := s.ctx.State().SchedulerConfig()

	// Check if preemption is enabled, defaults to true
	enablePreemption := true
	if schedConfig != nil {
		if s.job.Type == structs.JobTypeBatch {
			enablePreemption = schedConfig.PreemptionConfig.BatchSchedulerEnabled
		} else {
			enablePreemption = schedConfig.PreemptionConfig.ServiceSchedulerEnabled
		}
	}
	// Run stack again with preemption enabled
	if option == nil && enablePreemption {
		selectOptions.Preempt = true
		option = s.stack.Select(tg, selectOptions)
	}
	return option
}

// handlePreemptions sets relevant preeemption related fields. In OSS this is a no op.
func (s *GenericScheduler) handlePreemptions(option *RankedNode, alloc *structs.Allocation, missing placementResult) {
	if option.PreemptedAllocs == nil {
		return
	}

	// If this placement involves preemption, set DesiredState to evict for those allocations
	var preemptedAllocIDs []string
	for _, stop := range option.PreemptedAllocs {
		s.plan.AppendPreemptedAlloc(stop, alloc.ID)
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
