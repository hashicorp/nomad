package scheduler

import (
	"fmt"
	"log"

	memdb "github.com/hashicorp/go-memdb"
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

	// allocLost is the status used when an allocation is lost
	allocLost = "alloc is lost since its node is down"

	// allocInPlace is the status used when speculating on an in-place update
	allocInPlace = "alloc updating in-place"

	// blockedEvalMaxPlanDesc is the description used for blocked evals that are
	// a result of hitting the max number of plan attempts
	blockedEvalMaxPlanDesc = "created due to placement conflicts"

	// blockedEvalFailedPlacements is the description used for blocked evals
	// that are a result of failing to place all allocations.
	blockedEvalFailedPlacements = "created to place remaining allocations"
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

	deployment *structs.Deployment

	blocked        *structs.Evaluation
	failedTGAllocs map[string]*structs.AllocMetric
	queuedAllocs   map[string]int
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
		structs.EvalTriggerPeriodicJob, structs.EvalTriggerMaxPlans,
		structs.EvalTriggerDeploymentWatcher:
	default:
		desc := fmt.Sprintf("scheduler cannot handle '%s' evaluation reason",
			eval.TriggeredBy)
		return setStatus(s.logger, s.planner, s.eval, s.nextEval, s.blocked,
			s.failedTGAllocs, structs.EvalStatusFailed, desc, s.queuedAllocs)
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
			if err := s.createBlockedEval(true); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
			if err := setStatus(s.logger, s.planner, s.eval, s.nextEval, s.blocked,
				s.failedTGAllocs, statusErr.EvalStatus, err.Error(),
				s.queuedAllocs); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
			return mErr.ErrorOrNil()
		}
		return err
	}

	// If the current evaluation is a blocked evaluation and we didn't place
	// everything, do not update the status to complete.
	if s.eval.Status == structs.EvalStatusBlocked && len(s.failedTGAllocs) != 0 {
		e := s.ctx.Eligibility()
		newEval := s.eval.Copy()
		newEval.EscapedComputedClass = e.HasEscaped()
		newEval.ClassEligibility = e.GetClasses()
		return s.planner.ReblockEval(newEval)
	}

	// Update the status to complete
	return setStatus(s.logger, s.planner, s.eval, s.nextEval, s.blocked,
		s.failedTGAllocs, structs.EvalStatusComplete, "", s.queuedAllocs)
}

// createBlockedEval creates a blocked eval and submits it to the planner. If
// failure is set to true, the eval's trigger reason reflects that.
func (s *GenericScheduler) createBlockedEval(planFailure bool) error {
	e := s.ctx.Eligibility()
	escaped := e.HasEscaped()

	// Only store the eligible classes if the eval hasn't escaped.
	var classEligibility map[string]bool
	if !escaped {
		classEligibility = e.GetClasses()
	}

	s.blocked = s.eval.CreateBlockedEval(classEligibility, escaped)
	if planFailure {
		s.blocked.TriggeredBy = structs.EvalTriggerMaxPlans
		s.blocked.StatusDescription = blockedEvalMaxPlanDesc
	} else {
		s.blocked.StatusDescription = blockedEvalFailedPlacements
	}

	return s.planner.CreateEval(s.blocked)
}

// process is wrapped in retryMax to iteratively run the handler until we have no
// further work or we've made the maximum number of attempts.
func (s *GenericScheduler) process() (bool, error) {
	// Lookup the Job by ID
	var err error
	ws := memdb.NewWatchSet()
	s.job, err = s.state.JobByID(ws, s.eval.JobID)
	if err != nil {
		return false, fmt.Errorf("failed to get job %q: %v", s.eval.JobID, err)
	}

	numTaskGroups := 0
	stopped := s.job.Stopped()
	if !stopped {
		numTaskGroups = len(s.job.TaskGroups)
	}
	s.queuedAllocs = make(map[string]int, numTaskGroups)

	// Create a plan
	s.plan = s.eval.MakePlan(s.job)

	if !s.batch {
		// Get any existing deployment
		s.deployment, err = s.state.LatestDeploymentByJobID(ws, s.eval.JobID)
		if err != nil {
			return false, fmt.Errorf("failed to get job deployment %q: %v", s.eval.JobID, err)
		}
	}

	// Reset the failed allocations
	s.failedTGAllocs = nil

	// Create an evaluation context
	s.ctx = NewEvalContext(s.state, s.plan, s.logger)

	// Construct the placement stack
	s.stack = NewGenericStack(s.batch, s.ctx)
	if !s.job.Stopped() {
		s.stack.SetJob(s.job)
	}

	// Compute the target job allocations
	if err := s.computeJobAllocs(); err != nil {
		s.logger.Printf("[ERR] sched: %#v: %v", s.eval, err)
		return false, err
	}

	// If there are failed allocations, we need to create a blocked evaluation
	// to place the failed allocations when resources become available. If the
	// current evaluation is already a blocked eval, we reuse it.
	if s.eval.Status != structs.EvalStatusBlocked && len(s.failedTGAllocs) != 0 && s.blocked == nil {
		if err := s.createBlockedEval(false); err != nil {
			s.logger.Printf("[ERR] sched: %#v failed to make blocked eval: %v", s.eval, err)
			return false, err
		}
		s.logger.Printf("[DEBUG] sched: %#v: failed to place all allocations, blocked eval '%s' created", s.eval, s.blocked.ID)
	}

	// If the plan is a no-op, we can bail. If AnnotatePlan is set submit the plan
	// anyways to get the annotations.
	if s.plan.IsNoOp() && !s.eval.AnnotatePlan {
		return true, nil
	}

	// XXX Don't need a next rolling update eval
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

	// Submit the plan and store the results.
	result, newState, err := s.planner.SubmitPlan(s.plan)
	s.planResult = result
	if err != nil {
		return false, err
	}

	// Decrement the number of allocations pending per task group based on the
	// number of allocations successfully placed
	adjustQueuedAllocations(s.logger, result, s.queuedAllocs)

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
func (s *GenericScheduler) filterCompleteAllocs(allocs []*structs.Allocation) ([]*structs.Allocation, map[string]*structs.Allocation) {
	filter := func(a *structs.Allocation) bool {
		if s.batch {
			// Allocs from batch jobs should be filtered when the desired status
			// is terminal and the client did not finish or when the client
			// status is failed so that they will be replaced. If they are
			// complete but not failed, they shouldn't be replaced.
			switch a.DesiredStatus {
			case structs.AllocDesiredStatusStop, structs.AllocDesiredStatusEvict:
				return !a.RanSuccessfully()
			default:
			}

			switch a.ClientStatus {
			case structs.AllocClientStatusFailed:
				return true
			default:
				return false
			}
		}

		// Filter terminal, non batch allocations
		return a.TerminalStatus()
	}

	terminalAllocsByName := make(map[string]*structs.Allocation)
	n := len(allocs)
	for i := 0; i < n; i++ {
		if filter(allocs[i]) {

			// Add the allocation to the terminal allocs map if it's not already
			// added or has a higher create index than the one which is
			// currently present.
			alloc, ok := terminalAllocsByName[allocs[i].Name]
			if !ok || alloc.CreateIndex < allocs[i].CreateIndex {
				terminalAllocsByName[allocs[i].Name] = allocs[i]
			}

			// Remove the allocation
			allocs[i], allocs[n-1] = allocs[n-1], nil
			i--
			n--
		}
	}

	// If the job is batch, we want to filter allocations that have been
	// replaced by a newer version for the same task group.
	filtered := allocs[:n]
	if s.batch {
		byTG := make(map[string]*structs.Allocation)
		for _, alloc := range filtered {
			existing := byTG[alloc.Name]
			if existing == nil || existing.CreateIndex < alloc.CreateIndex {
				byTG[alloc.Name] = alloc
			}
		}

		filtered = make([]*structs.Allocation, 0, len(byTG))
		for _, alloc := range byTG {
			filtered = append(filtered, alloc)
		}
	}

	return filtered, terminalAllocsByName
}

// computeJobAllocs is used to reconcile differences between the job,
// existing allocations and node status to update the allocations.
func (s *GenericScheduler) computeJobAllocs() error {
	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	allocs, err := s.state.AllocsByJob(ws, s.eval.JobID, true)
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

	// Update the allocations which are in pending/running state on tainted
	// nodes to lost
	updateNonTerminalAllocsToLost(s.plan, tainted, allocs)

	// Filter out the allocations in a terminal state
	allocs, _ = s.filterCompleteAllocs(allocs)

	reconciler := NewAllocReconciler(s.ctx.Logger(),
		genericAllocUpdateFn(s.ctx, s.stack, s.eval.ID),
		s.batch, s.eval.JobID, s.job, s.deployment, allocs, tainted)
	results := reconciler.Compute()

	if s.eval.AnnotatePlan {
		s.plan.Annotations = &structs.PlanAnnotations{
			DesiredTGUpdates: results.desiredTGUpdates,
		}
	}

	// Add the deployment changes to the plan
	s.plan.CreatedDeployment = results.createDeployment
	s.plan.DeploymentUpdates = results.deploymentUpdates

	// Update the stored deployment
	if results.createDeployment != nil {
		s.deployment = results.createDeployment
	}

	// Handle the stop
	for _, stop := range results.stop {
		s.plan.AppendUpdate(stop.alloc, structs.AllocDesiredStatusStop, stop.statusDescription, stop.clientStatus)
	}

	// Handle the in-place updates
	for _, update := range results.inplaceUpdate {
		s.ctx.Plan().AppendAlloc(update)
	}

	// Nothing remaining to do if placement is not required
	if len(results.place) == 0 {
		if !s.job.Stopped() {
			for _, tg := range s.job.TaskGroups {
				s.queuedAllocs[tg.Name] = 0
			}
		}
		return nil
	}

	// Record the number of allocations that needs to be placed per Task Group
	for _, place := range results.place {
		s.queuedAllocs[place.taskGroup.Name] += 1
	}

	// Compute the placements
	return s.computePlacements(results.place)
}

// computePlacements computes placements for allocations
func (s *GenericScheduler) computePlacements(place []allocPlaceResult) error {
	// Get the base nodes
	nodes, byDC, err := readyNodesInDCs(s.state, s.job.Datacenters)
	if err != nil {
		return err
	}

	var deploymentID string
	if s.deployment != nil {
		deploymentID = s.deployment.ID
	}

	// Update the set of placement ndoes
	s.stack.SetNodes(nodes)

	for _, missing := range place {
		// Check if this task group has already failed
		if metric, ok := s.failedTGAllocs[missing.taskGroup.Name]; ok {
			metric.CoalescedFailures += 1
			continue
		}

		// Find the preferred node
		preferredNode, err := s.findPreferredNode(&missing)
		if err != nil {
			return err
		}

		// Attempt to match the task group
		var option *RankedNode
		if preferredNode != nil {
			option, _ = s.stack.SelectPreferringNodes(missing.taskGroup, []*structs.Node{preferredNode})
		} else {
			option, _ = s.stack.Select(missing.taskGroup)
		}

		// Store the available nodes by datacenter
		s.ctx.Metrics().NodesAvailable = byDC

		// Set fields based on if we found an allocation option
		if option != nil {
			// Create an allocation for this
			alloc := &structs.Allocation{
				ID:            structs.GenerateUUID(),
				EvalID:        s.eval.ID,
				Name:          missing.name,
				JobID:         s.job.ID,
				TaskGroup:     missing.taskGroup.Name,
				Metrics:       s.ctx.Metrics(),
				NodeID:        option.Node.ID,
				DeploymentID:  deploymentID,
				Canary:        missing.canary,
				TaskResources: option.TaskResources,
				DesiredStatus: structs.AllocDesiredStatusRun,
				ClientStatus:  structs.AllocClientStatusPending,

				SharedResources: &structs.Resources{
					DiskMB: missing.taskGroup.EphemeralDisk.SizeMB,
				},
			}

			// If the new allocation is replacing an older allocation then we
			// set the record the older allocation id so that they are chained
			if missing.previousAlloc != nil {
				alloc.PreviousAllocation = missing.previousAlloc.ID
			}

			s.plan.AppendAlloc(alloc)
		} else {
			// Lazy initialize the failed map
			if s.failedTGAllocs == nil {
				s.failedTGAllocs = make(map[string]*structs.AllocMetric)
			}

			s.failedTGAllocs[missing.taskGroup.Name] = s.ctx.Metrics()
		}
	}

	return nil
}

// findPreferredNode finds the preferred node for an allocation
func (s *GenericScheduler) findPreferredNode(place *allocPlaceResult) (node *structs.Node, err error) {
	if place.previousAlloc != nil && place.taskGroup.EphemeralDisk.Sticky == true {
		var preferredNode *structs.Node
		ws := memdb.NewWatchSet()
		preferredNode, err = s.state.NodeByID(ws, place.previousAlloc.NodeID)
		if preferredNode.Ready() {
			node = preferredNode
		}
	}
	return
}
