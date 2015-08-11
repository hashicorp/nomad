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
	// Lookup the Job by ID
	job, err := s.state.GetJobByID(eval.JobID)
	if err != nil {
		return fmt.Errorf("failed to get job '%s': %v",
			eval.JobID, err)
	}

	// If the job is missing, maybe a concurrent deregister
	if job == nil {
		s.logger.Printf("[DEBUG] sched: skipping eval %s, job %s not found",
			eval.ID, eval.JobID)
		return nil
	}

	// Materialize all the task groups
	groups := materializeTaskGroups(job)

	// If there is nothing required for this job, treat like a deregister
	if len(groups) == 0 {
		return s.handleJobDeregister(eval)
	}

	// Lookup the allocations by JobID
	allocs, err := s.state.AllocsByJob(eval.JobID)
	if err != nil {
		return fmt.Errorf("failed to get allocs for job '%s': %v",
			eval.JobID, err)
	}

	// Index the existing allocations
	indexed := indexAllocs(allocs)

	// Diff the required and existing allocations
	place, update, evict, ignore := diffAllocs(job, groups, indexed)
	s.logger.Printf("[DEBUG] sched: eval %s job %s needs %d placements, %d updates, %d evictions, %d ignored allocs",
		eval.ID, eval.JobID, len(place), len(update), len(evict), len(ignore))

	// Fast-pass if nothing to do
	if len(place) == 0 && len(update) == 0 && len(evict) == 0 {
		return nil
	}

	// Start a plan for this evaluation
	plan := eval.MakePlan(job)

	// Add all the evicts
	addEvictsToPlan(plan, evict, indexed)

	// For simplicity, we treat all updates as an evict + place.
	// XXX: This should be done with rolling in-place updates instead.
	addEvictsToPlan(plan, update, indexed)
	place = append(place, update...)

	// Attempt to place all the allocations
	planAllocations(job, plan, place, groups)

	// TODO
	return nil
}

func planAllocations(job *structs.Job, plan *structs.Plan,
	place []allocNameID, groups map[string]*structs.TaskGroup) {
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
	s.logger.Printf("[DEBUG] sched: eval %s job %s needs %d evictions",
		eval.ID, eval.JobID, len(allocs))
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

// handleNodeUpdate is used to handle an update to a node status where
// there is an existing allocation for this job
func (s *ServiceScheduler) handleNodeUpdate(eval *structs.Evaluation) error {
	// TODO
	return nil
}

// materializeTaskGroups is used to materialize all the task groups
// a job requires. This is used to do the count expansion.
func materializeTaskGroups(job *structs.Job) map[string]*structs.TaskGroup {
	out := make(map[string]*structs.TaskGroup)
	for _, tg := range job.TaskGroups {
		for i := 0; i < tg.Count; i++ {
			name := fmt.Sprintf("%s.%s[%d]", job.Name, tg.Name, i)
			out[name] = tg
		}
	}
	return out
}

// indexAllocs is used to index a list of allocations by name
func indexAllocs(allocs []*structs.Allocation) map[string][]*structs.Allocation {
	out := make(map[string][]*structs.Allocation)
	for _, alloc := range allocs {
		name := alloc.Name
		out[name] = append(out[name], alloc)
	}
	return out
}

// allocNameID is a tuple of the allocation name and ID
type allocNameID struct {
	Name string
	ID   string
}

// diffAllocs is used to do a set difference between the target allocations
// and the existing allocations. This returns 4 sets of results, the list of
// named task groups that need to be placed (no existing allocation), the
// allocations that need to be updated (job definition is newer), the allocs
// that need to be evicted (no longer required), and those that should be
// ignored.
func diffAllocs(job *structs.Job,
	required map[string]*structs.TaskGroup,
	existing map[string][]*structs.Allocation) (place, update, evict, ignore []allocNameID) {
	// Scan the existing updates
	for name, existList := range existing {
		for _, exist := range existList {
			// Check for the definition in the required set
			_, ok := required[name]

			// If not required, we evict
			if !ok {
				evict = append(evict, allocNameID{name, exist.ID})
				continue
			}

			// If the definition is updated we need to update
			// XXX: This is an extremely conservative approach. We can check
			// if the job definition has changed in a way that affects
			// this allocation and potentially ignore it.
			if job.ModifyIndex != exist.Job.ModifyIndex {
				update = append(update, allocNameID{name, exist.ID})
				continue
			}

			// Everything is up-to-date
			ignore = append(ignore, allocNameID{name, exist.ID})
		}
	}

	// Scan the required groups
	for name := range required {
		// Check for an existing allocation
		_, ok := existing[name]

		// Require a placement if no existing allocation. If there
		// is an existing allocation, we would have checked for a potential
		// update or ignore above.
		if !ok {
			place = append(place, allocNameID{name, ""})
		}
	}
	return
}

// addEvictsToPlan is used to add all the evictions to the plan
func addEvictsToPlan(plan *structs.Plan,
	evicts []allocNameID, indexed map[string][]*structs.Allocation) {
	for _, evict := range evicts {
		list := indexed[evict.Name]
		for _, alloc := range list {
			if alloc.ID != evict.ID {
				continue
			}

			// Add this eviction to the per-node list
			nodeEvict := plan.NodeEvict[alloc.NodeID]
			nodeEvict = append(nodeEvict, evict.ID)
			plan.NodeEvict[alloc.NodeID] = nodeEvict
		}
	}
}
