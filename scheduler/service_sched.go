package scheduler

import (
	"fmt"
	"log"

	"github.com/hashicorp/nomad/nomad/structs"
)

// ServiceScheduler is used for 'service' type jobs. This scheduler is
// designed for long-lived services, and as such spends more time attemping
// to make a high quality placement. This is the primary scheduler for
// most workloads.
type ServiceScheduler struct {
	logger  *log.Logger
	state   State
	planner Planner
}

// NewServiceScheduler is a factory function to instantiate a new service scheduler
func NewServiceScheduler(logger *log.Logger, state State, planner Planner) Scheduler {
	s := &ServiceScheduler{
		logger:  logger,
		state:   state,
		planner: planner,
	}
	return s
}

// Process is used to handle a single evaluation
func (s *ServiceScheduler) Process(eval *structs.Evaluation) error {
	// Use the evaluation trigger reason to determine what we need to do
	switch eval.TriggeredBy {
	case structs.EvalTriggerJobRegister:
		return s.handleJobRegister(eval)
	case structs.EvalTriggerJobDeregister:
		return s.handleJobDeregister(eval)
	case structs.EvalTriggerNodeUpdate:
		return s.handleNodeUpdate(eval)
	default:
		return fmt.Errorf("service scheduler cannot handle '%s' evaluation reason",
			eval.TriggeredBy)
	}
}

// handleJobRegister is used to handle a job being registered or updated
func (s *ServiceScheduler) handleJobRegister(eval *structs.Evaluation) error {
	// TODO
	return nil
}

// handleJobDeregister is used to handle a job being deregistered
func (s *ServiceScheduler) handleJobDeregister(eval *structs.Evaluation) error {
START:
	// Lookup the allocations by JobID
	allocs, err := s.state.AllocsByJob(eval.JobID)
	if err != nil {
		return fmt.Errorf("failed to get allocs for job '%s': %v",
			eval.JobID, err)
	}

	// Nothing to do if there is no evictsion
	if len(allocs) == 0 {
		return nil
	}

	// Create a plan to evict these
	plan := &structs.Plan{
		EvalID:    eval.ID,
		Priority:  eval.Priority,
		NodeEvict: make(map[string][]string),
	}

	// Add each alloc to be evicted
	for _, alloc := range allocs {
		nodeEvict := plan.NodeEvict[alloc.NodeID]
		nodeEvict = append(nodeEvict, alloc.ID)
		plan.NodeEvict[alloc.NodeID] = nodeEvict
	}

	// Submit the plan
	_, newState, err := s.planner.SubmitPlan(plan)
	if err != nil {
		return err
	}

	// If we got a state refresh, try again to ensure we
	// are not missing any allocations
	if newState != nil {
		s.state = newState
		goto START
	}
	return nil
}

// handleNodeUPdate is used to handle an update to a node status where
// there is an existing allocation for this job
func (s *ServiceScheduler) handleNodeUpdate(eval *structs.Evaluation) error {
	// TODO
	return nil
}
