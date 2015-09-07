package scheduler

import (
	"fmt"
	"log"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// maxServiceScheduleAttempts is used to limit the number of times
	// we will attempt to schedule if we continue to hit conflicts for services.
	maxServiceScheduleAttempts = 5

	// maxBatchScheduleAttempts is used to limit the number of times
	// we will attempt to schedule if we continue to hit conflicts for batch.
	maxBatchScheduleAttempts = 2

	// allocNotNeeded is the status used when a job no longer requires an allocation
	allocNotNeeded = "alloc not needed due to job update"

	// allocMigrating is the status used when we must migrate an allocation
	allocMigrating = "alloc is being migrated"

	// allocUpdating is the status used when a job requires an update
	allocUpdating = "alloc is being updated due to job update"

	// allocInPlace is the status used when speculating on an in-place update
	allocInPlace = "alloc updating in-place"
)

// SetStatusError is used to set the status of the evaluation to the given error
type SetStatusError struct {
	Err        error
	EvalStatus string
}

func (s *SetStatusError) Error() string {
	return s.Err.Error()
}

// GenericScheduler is used for 'service' and 'batch' type jobs. This scheduler is
// designed for long-lived services, and as such spends more time attemping
// to make a high quality placement. This is the primary scheduler for
// most workloads. It also supports a 'batch' mode to optimize for fast decision
// making at the cost of quality.
type GenericScheduler struct {
	logger  *log.Logger
	state   State
	planner Planner
	batch   bool

	eval  *structs.Evaluation
	job   *structs.Job
	plan  *structs.Plan
	ctx   *EvalContext
	stack *GenericStack
}

// NewServiceScheduler is a factory function to instantiate a new service scheduler
func NewServiceScheduler(logger *log.Logger, state State, planner Planner) Scheduler {
	s := &GenericScheduler{
		logger:  logger,
		state:   state,
		planner: planner,
		batch:   false,
	}
	return s
}

// NewBatchScheduler is a factory function to instantiate a new batch scheduler
func NewBatchScheduler(logger *log.Logger, state State, planner Planner) Scheduler {
	s := &GenericScheduler{
		logger:  logger,
		state:   state,
		planner: planner,
		batch:   true,
	}
	return s
}

// setStatus is used to update the status of the evaluation
func (s *GenericScheduler) setStatus(status, desc string) error {
	s.logger.Printf("[DEBUG] sched: %#v: setting status to %s", s.eval, status)
	newEval := s.eval.Copy()
	newEval.Status = status
	newEval.StatusDescription = desc
	return s.planner.UpdateEval(newEval)
}

// Process is used to handle a single evaluation
func (s *GenericScheduler) Process(eval *structs.Evaluation) error {
	// Verify the evaluation trigger reason is understood
	switch eval.TriggeredBy {
	case structs.EvalTriggerJobRegister, structs.EvalTriggerNodeUpdate,
		structs.EvalTriggerJobDeregister:
	default:
		desc := fmt.Sprintf("scheduler cannot handle '%s' evaluation reason",
			eval.TriggeredBy)
		return s.setStatus(structs.EvalStatusFailed, desc)
	}

	// Store the evaluation
	s.eval = eval

	// Retry up to the maxScheduleAttempts
	limit := maxServiceScheduleAttempts
	if s.batch {
		limit = maxBatchScheduleAttempts
	}
	if err := retryMax(limit, s.process); err != nil {
		if statusErr, ok := err.(*SetStatusError); ok {
			return s.setStatus(statusErr.EvalStatus, err.Error())
		}
		return err
	}

	// Update the status to complete
	return s.setStatus(structs.EvalStatusComplete, "")
}

// process is wrapped in retryMax to iteratively run the handler until we have no
// further work or we've made the maximum number of attempts.
func (s *GenericScheduler) process() (bool, error) {
	// Lookup the Job by ID
	var err error
	s.job, err = s.state.JobByID(s.eval.JobID)
	if err != nil {
		return false, fmt.Errorf("failed to get job '%s': %v",
			s.eval.JobID, err)
	}

	// Create a plan
	s.plan = s.eval.MakePlan(s.job)

	// Create an evaluation context
	s.ctx = NewEvalContext(s.state, s.plan, s.logger)

	// Construct the placement stack
	s.stack = NewGenericStack(s.batch, s.ctx, nil)
	if s.job != nil {
		s.stack.SetJob(s.job)
	}

	// Compute the target job allocations
	if err := s.computeJobAllocs(); err != nil {
		s.logger.Printf("[ERR] sched: %#v: %v", s.eval, err)
		return false, err
	}

	// If the plan is a no-op, we can bail
	if s.plan.IsNoOp() {
		return true, nil
	}

	// Submit the plan
	result, newState, err := s.planner.SubmitPlan(s.plan)
	if err != nil {
		return false, err
	}

	// If we got a state refresh, try again since we have stale data
	if newState != nil {
		s.logger.Printf("[DEBUG] sched: %#v: refresh forced", s.eval)
		s.state = newState
		return false, nil
	}

	// Try again if the plan was not fully committed, potential conflict
	fullCommit, expected, actual := result.FullCommit(s.plan)
	if !fullCommit {
		s.logger.Printf("[DEBUG] sched: %#v: attempted %d placements, %d placed",
			s.eval, expected, actual)
		return false, nil
	}

	// Success!
	return true, nil
}

// computeJobAllocs is used to reconcile differences between the job,
// existing allocations and node status to update the allocations.
func (s *GenericScheduler) computeJobAllocs() error {
	// Materialize all the task groups, job could be missing if deregistered
	var groups map[string]*structs.TaskGroup
	if s.job != nil {
		groups = materializeTaskGroups(s.job)
	}

	// Lookup the allocations by JobID
	allocs, err := s.state.AllocsByJob(s.eval.JobID)
	if err != nil {
		return fmt.Errorf("failed to get allocs for job '%s': %v",
			s.eval.JobID, err)
	}

	// Determine the tainted nodes containing job allocs
	tainted, err := taintedNodes(s.state, allocs)
	if err != nil {
		return fmt.Errorf("failed to get tainted nodes for job '%s': %v",
			s.eval.JobID, err)
	}

	// Diff the required and existing allocations
	diff := diffAllocs(s.job, tainted, groups, allocs)
	s.logger.Printf("[DEBUG] sched: %#v: %#v", s.eval, diff)

	// Add all the allocs to stop
	for _, e := range diff.stop {
		s.plan.AppendUpdate(e.Alloc, structs.AllocDesiredStatusStop, allocNotNeeded)
	}

	// Attempt to do the upgrades in place
	diff.update = s.inplaceUpdate(diff.update)

	// Any updates not possible inplace we treat all as an evict + place.
	// XXX: This should be done with rolling in-place updates instead.
	for _, e := range diff.update {
		s.plan.AppendUpdate(e.Alloc, structs.AllocDesiredStatusStop, allocUpdating)
	}
	diff.place = append(diff.place, diff.update...)

	// For simplicity, we treat all migrates as an evict + place.
	// XXX: This could probably be done more intelligently?
	for _, e := range diff.migrate {
		s.plan.AppendUpdate(e.Alloc, structs.AllocDesiredStatusStop, allocMigrating)
	}
	diff.place = append(diff.place, diff.migrate...)

	// Nothing remaining to do if placement is not required
	if len(diff.place) == 0 {
		return nil
	}

	// Compute the placements
	return s.computePlacements(diff.place)
}

// inplaceUpdate attempts to update allocations in-place where possible.
func (s *GenericScheduler) inplaceUpdate(updates []allocTuple) []allocTuple {
	n := len(updates)
	inplace := 0
	for i := 0; i < n; i++ {
		// Get the udpate
		update := updates[i]

		// Check if the task drivers or config has changed, requires
		// a rolling upgrade since that cannot be done in-place.
		existing := update.Alloc.Job.LookupTaskGroup(update.TaskGroup.Name)
		if tasksUpdated(update.TaskGroup, existing) {
			continue
		}

		// Get the existing node
		node, err := s.state.NodeByID(update.Alloc.NodeID)
		if err != nil {
			s.logger.Printf("[ERR] sched: %#v failed to get node '%s': %v",
				update.Alloc.NodeID, err)
			continue
		}
		if node == nil {
			continue
		}

		// Set the existing node as the base set
		s.stack.SetNodes([]*structs.Node{node})

		// Stage an eviction of the current allocation
		s.plan.AppendUpdate(update.Alloc, structs.AllocDesiredStatusStop,
			allocInPlace)

		// Attempt to match the task group
		option, size := s.stack.Select(update.TaskGroup)

		// Pop the allocation
		s.plan.PopUpdate(update.Alloc)

		// Skip if we could not do an in-place update
		if option == nil {
			continue
		}

		// Create a shallow copy
		newAlloc := new(structs.Allocation)
		*newAlloc = *update.Alloc

		// Update the allocation
		newAlloc.EvalID = s.eval.ID
		newAlloc.Job = s.job
		newAlloc.Resources = size
		newAlloc.Metrics = s.ctx.Metrics()
		newAlloc.DesiredStatus = structs.AllocDesiredStatusRun
		newAlloc.ClientStatus = structs.AllocClientStatusPending
		s.plan.AppendAlloc(newAlloc)

		// Remove this allocation from the slice
		updates[i] = updates[n-1]
		i--
		n--
		inplace++
	}
	if len(updates) > 0 {
		s.logger.Printf("[DEBUG] sched: %#v: %d in-place updates of %d", s.eval, inplace, len(updates))
	}
	return updates[:n]
}

// computePlacements computes placements for allocations
func (s *GenericScheduler) computePlacements(place []allocTuple) error {
	// Get the base nodes
	nodes, err := readyNodesInDCs(s.state, s.job.Datacenters)
	if err != nil {
		return err
	}

	// Update the set of placement ndoes
	s.stack.SetNodes(nodes)

	// Track the failed task groups so that we can coalesce
	// the failures together to avoid creating many failed allocs.
	failedTG := make(map[*structs.TaskGroup]*structs.Allocation)

	for _, missing := range place {
		// Check if this task group has already failed
		if alloc, ok := failedTG[missing.TaskGroup]; ok {
			alloc.Metrics.CoalescedFailures += 1
			continue
		}

		// Attempt to match the task group
		option, size := s.stack.Select(missing.TaskGroup)

		// Handle a placement failure
		var nodeID, status, desc, clientStatus string
		if option == nil {
			status = structs.AllocDesiredStatusFailed
			desc = "failed to find a node for placement"
			clientStatus = structs.AllocClientStatusFailed
		} else {
			nodeID = option.Node.ID
			status = structs.AllocDesiredStatusRun
			clientStatus = structs.AllocClientStatusPending
		}

		// Create an allocation for this
		alloc := &structs.Allocation{
			ID:                 mock.GenerateUUID(),
			EvalID:             s.eval.ID,
			Name:               missing.Name,
			NodeID:             nodeID,
			JobID:              s.job.ID,
			Job:                s.job,
			TaskGroup:          missing.TaskGroup.Name,
			Resources:          size,
			Metrics:            s.ctx.Metrics(),
			DesiredStatus:      status,
			DesiredDescription: desc,
			ClientStatus:       clientStatus,
		}
		if nodeID != "" {
			s.plan.AppendAlloc(alloc)
		} else {
			s.plan.AppendFailed(alloc)
			failedTG[missing.TaskGroup] = alloc
		}
	}
	return nil
}
