package scheduler

import (
	"fmt"
	"log"

	"github.com/hashicorp/go-multierror"
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

	eval       *structs.Evaluation
	job        *structs.Job
	plan       *structs.Plan
	planResult *structs.PlanResult
	ctx        *EvalContext
	stack      *GenericStack

	limitReached bool
	nextEval     *structs.Evaluation

	blocked *structs.Evaluation
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

// Process is used to handle a single evaluation
func (s *GenericScheduler) Process(eval *structs.Evaluation) error {
	// Store the evaluation
	s.eval = eval

	// Verify the evaluation trigger reason is understood
	switch eval.TriggeredBy {
	case structs.EvalTriggerJobRegister, structs.EvalTriggerNodeUpdate,
		structs.EvalTriggerJobDeregister, structs.EvalTriggerRollingUpdate,
		structs.EvalTriggerPeriodicJob:
	default:
		desc := fmt.Sprintf("scheduler cannot handle '%s' evaluation reason",
			eval.TriggeredBy)
		return setStatus(s.logger, s.planner, s.eval, s.nextEval, structs.EvalStatusFailed, desc)
	}

	// Retry up to the maxScheduleAttempts and reset if progress is made.
	progress := func() bool { return progressMade(s.planResult) }
	limit := maxServiceScheduleAttempts
	if s.batch {
		limit = maxBatchScheduleAttempts
	}
	if err := retryMax(limit, s.process, progress); err != nil {
		if statusErr, ok := err.(*SetStatusError); ok {
			// Scheduling was tried but made no forward progress so create a
			// blocked eval to retry once resources become available.
			var mErr multierror.Error
			if err := s.createBlockedEval(); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
			if err := setStatus(s.logger, s.planner, s.eval, s.nextEval, statusErr.EvalStatus, err.Error()); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
			return mErr.ErrorOrNil()
		}
		return err
	}

	// Update the status to complete
	return setStatus(s.logger, s.planner, s.eval, s.nextEval, structs.EvalStatusComplete, "")
}

// createBlockedEval creates a blocked eval and stores it.
func (s *GenericScheduler) createBlockedEval() error {
	e := s.ctx.Eligibility()
	escaped := e.HasEscaped()

	// Only store the eligible classes if the eval hasn't escaped.
	var classEligibility map[string]bool
	if !escaped {
		classEligibility = e.GetClasses()
	}

	s.blocked = s.eval.BlockedEval(classEligibility, escaped)
	return s.planner.CreateEval(s.blocked)
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
	s.stack = NewGenericStack(s.batch, s.ctx)
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

	// If the limit of placements was reached we need to create an evaluation
	// to pickup from here after the stagger period.
	if s.limitReached && s.nextEval == nil {
		s.nextEval = s.eval.NextRollingEval(s.job.Update.Stagger)
		if err := s.planner.CreateEval(s.nextEval); err != nil {
			s.logger.Printf("[ERR] sched: %#v failed to make next eval for rolling update: %v", s.eval, err)
			return false, err
		}
		s.logger.Printf("[DEBUG] sched: %#v: rolling update limit reached, next eval '%s' created", s.eval, s.nextEval.ID)
	}

	// If there are failed allocations, we need to create a blocked evaluation
	// to place the failed allocations when resources become available.
	if len(s.plan.FailedAllocs) != 0 && s.blocked == nil {
		if err := s.createBlockedEval(); err != nil {
			s.logger.Printf("[ERR] sched: %#v failed to make blocked eval: %v", s.eval, err)
			return false, err
		}
		s.logger.Printf("[DEBUG] sched: %#v: failed to place all allocations, blocked eval '%s' created", s.eval, s.blocked.ID)
	}

	// Submit the plan and store the results.
	result, newState, err := s.planner.SubmitPlan(s.plan)
	s.planResult = result
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
		if newState == nil {
			return false, fmt.Errorf("missing state refresh after partial commit")
		}
		return false, nil
	}

	// Success!
	return true, nil
}

// filterCompleteAllocs filters allocations that are terminal and should be
// re-placed.
func (s *GenericScheduler) filterCompleteAllocs(allocs []*structs.Allocation) []*structs.Allocation {
	filter := func(a *structs.Allocation) bool {
		// Allocs from batch jobs should be filtered when their status is failed so that
		// they will be replaced. If they are dead but not failed, they
		// shouldn't be replaced.
		if s.batch {
			return a.ClientStatus == structs.AllocClientStatusFailed
		}

		// Filter terminal, non batch allocations
		return a.TerminalStatus()
	}

	n := len(allocs)
	for i := 0; i < n; i++ {
		if filter(allocs[i]) {
			allocs[i], allocs[n-1] = allocs[n-1], nil
			i--
			n--
		}
	}
	return allocs[:n]
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

	// Filter out the allocations in a terminal state
	allocs = s.filterCompleteAllocs(allocs)

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
	diff.update = inplaceUpdate(s.ctx, s.eval, s.job, s.stack, diff.update)

	// Check if a rolling upgrade strategy is being used
	limit := len(diff.update) + len(diff.migrate)
	if s.job != nil && s.job.Update.Rolling() {
		limit = s.job.Update.MaxParallel
	}

	// Treat migrations as an eviction and a new placement.
	s.limitReached = evictAndPlace(s.ctx, diff, diff.migrate, allocMigrating, &limit)

	// Treat non in-place updates as an eviction and new placement.
	s.limitReached = s.limitReached || evictAndPlace(s.ctx, diff, diff.update, allocUpdating, &limit)

	// Nothing remaining to do if placement is not required
	if len(diff.place) == 0 {
		return nil
	}

	// Compute the placements
	return s.computePlacements(diff.place)
}

// computePlacements computes placements for allocations
func (s *GenericScheduler) computePlacements(place []allocTuple) error {
	// Get the base nodes
	nodes, byDC, err := readyNodesInDCs(s.state, s.job.Datacenters)
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
		option, _ := s.stack.Select(missing.TaskGroup)

		// Create an allocation for this
		alloc := &structs.Allocation{
			ID:        structs.GenerateUUID(),
			EvalID:    s.eval.ID,
			Name:      missing.Name,
			JobID:     s.job.ID,
			TaskGroup: missing.TaskGroup.Name,
			Metrics:   s.ctx.Metrics(),
		}

		// Store the available nodes by datacenter
		s.ctx.Metrics().NodesAvailable = byDC

		// Set fields based on if we found an allocation option
		if option != nil {
			// Generate service IDs tasks in this allocation
			alloc.PopulateServiceIDs(missing.TaskGroup)

			alloc.NodeID = option.Node.ID
			alloc.TaskResources = option.TaskResources
			alloc.DesiredStatus = structs.AllocDesiredStatusRun
			alloc.ClientStatus = structs.AllocClientStatusPending
			s.plan.AppendAlloc(alloc)
		} else {
			alloc.DesiredStatus = structs.AllocDesiredStatusFailed
			alloc.DesiredDescription = "failed to find a node for placement"
			alloc.ClientStatus = structs.AllocClientStatusFailed
			s.plan.AppendFailed(alloc)
			failedTG[missing.TaskGroup] = alloc
		}
	}

	return nil
}
