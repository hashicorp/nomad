// +build !pro,!ent

package scheduler

import "github.com/hashicorp/nomad/nomad/structs"

// selectNextOption calls the stack to get a node for placement
func (s *GenericScheduler) selectNextOption(tg *structs.TaskGroup, selectOptions *SelectOptions) *RankedNode {
	return s.stack.Select(tg, selectOptions)
}

// handlePreemptions sets relevant preeemption related fields. In OSS this is a no op.
func (s *GenericScheduler) handlePreemptions(option *RankedNode, alloc *structs.Allocation, missing placementResult) {

}
