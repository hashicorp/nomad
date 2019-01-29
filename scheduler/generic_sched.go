package scheduler

import (
	"fmt"
	"time"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper/uuid"
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

	// allocNodeTainted is the status used when stopping an alloc because it's
	// node is tainted.
	allocNodeTainted = "alloc not needed as node is tainted"

	// blockedEvalMaxPlanDesc is the description used for blocked evals that are
	// a result of hitting the max number of plan attempts
	blockedEvalMaxPlanDesc = "created due to placement conflicts"

	// blockedEvalFailedPlacements is the description used for blocked evals
	// that are a result of failing to place all allocations.
	blockedEvalFailedPlacements = "created to place remaining allocations"

	// reschedulingFollowupEvalDesc is the description used when creating follow
	// up evals for delayed rescheduling
	reschedulingFollowupEvalDesc = "created for delayed rescheduling"

	// maxPastRescheduleEvents is the maximum number of past reschedule event
	// that we track when unlimited rescheduling is enabled
	maxPastRescheduleEvents = 5
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
// designed for long-lived services, and as such spends more time attempting
// to make a high quality placement. This is the primary scheduler for
// most workloads. It also supports a 'batch' mode to optimize for fast decision
// making at the cost of quality.
type GenericScheduler struct {
	logger  log.Logger
	state   State
	planner Planner
	batch   bool

	eval       *structs.Evaluation
	job        *structs.Job
	plan       *structs.Plan
	planResult *structs.PlanResult
	ctx        *EvalContext
	stack      *GenericStack

	followUpEvals []*structs.Evaluation

	deployment *structs.Deployment

	blocked        *structs.Evaluation
	failedTGAllocs map[string]*structs.AllocMetric
	queuedAllocs   map[string]int
}

// NewServiceScheduler is a factory function to instantiate a new service scheduler
func NewServiceScheduler(logger log.Logger, state State, planner Planner) Scheduler {
	s := &GenericScheduler{
		logger:  logger.Named("service_sched"),
		state:   state,
		planner: planner,
		batch:   false,
	}
	return s
}

// NewBatchScheduler is a factory function to instantiate a new batch scheduler
func NewBatchScheduler(logger log.Logger, state State, planner Planner) Scheduler {
	s := &GenericScheduler{
		logger:  logger.Named("batch_sched"),
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

	// Update our logger with the eval's information
	s.logger = s.logger.With("eval_id", eval.ID, "job_id", eval.JobID, "namespace", eval.Namespace)

	// Verify the evaluation trigger reason is understood
	switch eval.TriggeredBy {
	case structs.EvalTriggerJobRegister, structs.EvalTriggerJobDeregister,
		structs.EvalTriggerNodeDrain, structs.EvalTriggerNodeUpdate,
		structs.EvalTriggerRollingUpdate, structs.EvalTriggerQueuedAllocs,
		structs.EvalTriggerPeriodicJob, structs.EvalTriggerMaxPlans,
		structs.EvalTriggerDeploymentWatcher, structs.EvalTriggerRetryFailedAlloc,
		structs.EvalTriggerFailedFollowUp, structs.EvalTriggerPreemption:
	default:
		desc := fmt.Sprintf("scheduler cannot handle '%s' evaluation reason",
			eval.TriggeredBy)
		return setStatus(s.logger, s.planner, s.eval, nil, s.blocked,
			s.failedTGAllocs, structs.EvalStatusFailed, desc, s.queuedAllocs,
			s.deployment.GetID())
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
			if err := setStatus(s.logger, s.planner, s.eval, nil, s.blocked,
				s.failedTGAllocs, statusErr.EvalStatus, err.Error(),
				s.queuedAllocs, s.deployment.GetID()); err != nil {
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
		newEval.QuotaLimitReached = e.QuotaLimitReached()
		return s.planner.ReblockEval(newEval)
	}

	// Update the status to complete
	return setStatus(s.logger, s.planner, s.eval, nil, s.blocked,
		s.failedTGAllocs, structs.EvalStatusComplete, "", s.queuedAllocs,
		s.deployment.GetID())
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

	s.blocked = s.eval.CreateBlockedEval(classEligibility, escaped, e.QuotaLimitReached())
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
	s.job, err = s.state.JobByID(ws, s.eval.Namespace, s.eval.JobID)
	if err != nil {
		return false, fmt.Errorf("failed to get job %q: %v", s.eval.JobID, err)
	}

	numTaskGroups := 0
	stopped := s.job.Stopped()
	if !stopped {
		numTaskGroups = len(s.job.TaskGroups)
	}
	s.queuedAllocs = make(map[string]int, numTaskGroups)
	s.followUpEvals = nil

	// Create a plan
	s.plan = s.eval.MakePlan(s.job)

	if !s.batch {
		// Get any existing deployment
		s.deployment, err = s.state.LatestDeploymentByJobID(ws, s.eval.Namespace, s.eval.JobID)
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
		s.logger.Error("failed to compute job allocations", "error", err)
		return false, err
	}

	// If there are failed allocations, we need to create a blocked evaluation
	// to place the failed allocations when resources become available. If the
	// current evaluation is already a blocked eval, we reuse it.
	if s.eval.Status != structs.EvalStatusBlocked && len(s.failedTGAllocs) != 0 && s.blocked == nil {
		if err := s.createBlockedEval(false); err != nil {
			s.logger.Error("failed to make blocked eval", "error", err)
			return false, err
		}
		s.logger.Debug("failed to place all allocations, blocked eval created", "blocked_eval_id", s.blocked.ID)
	}

	// If the plan is a no-op, we can bail. If AnnotatePlan is set submit the plan
	// anyways to get the annotations.
	if s.plan.IsNoOp() && !s.eval.AnnotatePlan {
		return true, nil
	}

	// Create follow up evals for any delayed reschedule eligible allocations
	if len(s.followUpEvals) > 0 {
		for _, eval := range s.followUpEvals {
			eval.PreviousEval = s.eval.ID
			// TODO(preetha) this should be batching evals before inserting them
			if err := s.planner.CreateEval(eval); err != nil {
				s.logger.Error("failed to make next eval for rescheduling", "error", err)
				return false, err
			}
			s.logger.Debug("found reschedulable allocs, followup eval created", "followup_eval_id", eval.ID)
		}
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
		s.logger.Debug("refresh forced")
		s.state = newState
		return false, nil
	}

	// Try again if the plan was not fully committed, potential conflict
	fullCommit, expected, actual := result.FullCommit(s.plan)
	if !fullCommit {
		s.logger.Debug("plan didn't fully commit", "attempted", expected, "placed", actual)
		if newState == nil {
			return false, fmt.Errorf("missing state refresh after partial commit")
		}
		return false, nil
	}

	// Success!
	return true, nil
}

// computeJobAllocs is used to reconcile differences between the job,
// existing allocations and node status to update the allocations.
func (s *GenericScheduler) computeJobAllocs() error {
	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	allocs, err := s.state.AllocsByJob(ws, s.eval.Namespace, s.eval.JobID, true)
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

	reconciler := NewAllocReconciler(s.logger,
		genericAllocUpdateFn(s.ctx, s.stack, s.eval.ID),
		s.batch, s.eval.JobID, s.job, s.deployment, allocs, tainted, s.eval.ID)
	results := reconciler.Compute()
	s.logger.Debug("reconciled current state with desired state", "results", log.Fmt("%#v", results))

	if s.eval.AnnotatePlan {
		s.plan.Annotations = &structs.PlanAnnotations{
			DesiredTGUpdates: results.desiredTGUpdates,
		}
	}

	// Add the deployment changes to the plan
	s.plan.Deployment = results.deployment
	s.plan.DeploymentUpdates = results.deploymentUpdates

	// Store all the follow up evaluations from rescheduled allocations
	if len(results.desiredFollowupEvals) > 0 {
		for _, evals := range results.desiredFollowupEvals {
			s.followUpEvals = append(s.followUpEvals, evals...)
		}
	}

	// Update the stored deployment
	if results.deployment != nil {
		s.deployment = results.deployment
	}

	// Handle the stop
	for _, stop := range results.stop {
		s.plan.AppendUpdate(stop.alloc, structs.AllocDesiredStatusStop, stop.statusDescription, stop.clientStatus)
	}

	// Handle the in-place updates
	for _, update := range results.inplaceUpdate {
		if update.DeploymentID != s.deployment.GetID() {
			update.DeploymentID = s.deployment.GetID()
			update.DeploymentStatus = nil
		}
		s.ctx.Plan().AppendAlloc(update)
	}

	// Handle the annotation updates
	for _, update := range results.attributeUpdates {
		s.ctx.Plan().AppendAlloc(update)
	}

	// Nothing remaining to do if placement is not required
	if len(results.place)+len(results.destructiveUpdate) == 0 {
		// If the job has been purged we don't have access to the job. Otherwise
		// set the queued allocs to zero. This is true if the job is being
		// stopped as well.
		if s.job != nil {
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
	for _, destructive := range results.destructiveUpdate {
		s.queuedAllocs[destructive.placeTaskGroup.Name] += 1
	}

	// Compute the placements
	place := make([]placementResult, 0, len(results.place))
	for _, p := range results.place {
		place = append(place, p)
	}

	destructive := make([]placementResult, 0, len(results.destructiveUpdate))
	for _, p := range results.destructiveUpdate {
		destructive = append(destructive, p)
	}
	return s.computePlacements(destructive, place)
}

// computePlacements computes placements for allocations. It is given the set of
// destructive updates to place and the set of new placements to place.
func (s *GenericScheduler) computePlacements(destructive, place []placementResult) error {
	// Get the base nodes
	nodes, byDC, err := readyNodesInDCs(s.state, s.job.Datacenters)
	if err != nil {
		return err
	}

	var deploymentID string
	if s.deployment != nil && s.deployment.Active() {
		deploymentID = s.deployment.ID
	}

	// Update the set of placement nodes
	s.stack.SetNodes(nodes)

	// Capture current time to use as the start time for any rescheduled allocations
	now := time.Now()

	// Have to handle destructive changes first as we need to discount their
	// resources. To understand this imagine the resources were reduced and the
	// count was scaled up.
	for _, results := range [][]placementResult{destructive, place} {
		for _, missing := range results {
			// Get the task group
			tg := missing.TaskGroup()

			// Check if this task group has already failed
			if metric, ok := s.failedTGAllocs[tg.Name]; ok {
				metric.CoalescedFailures += 1
				continue
			}

			// Find the preferred node
			preferredNode, err := s.findPreferredNode(missing)
			if err != nil {
				return err
			}

			// Check if we should stop the previous allocation upon successful
			// placement of its replacement. This allow atomic placements/stops. We
			// stop the allocation before trying to find a replacement because this
			// frees the resources currently used by the previous allocation.
			stopPrevAlloc, stopPrevAllocDesc := missing.StopPreviousAlloc()
			prevAllocation := missing.PreviousAllocation()
			if stopPrevAlloc {
				s.plan.AppendUpdate(prevAllocation, structs.AllocDesiredStatusStop, stopPrevAllocDesc, "")
			}

			// Compute penalty nodes for rescheduled allocs
			selectOptions := getSelectOptions(prevAllocation, preferredNode)
			option := s.stack.Select(tg, selectOptions)

			// Store the available nodes by datacenter
			s.ctx.Metrics().NodesAvailable = byDC

			// Compute top K scoring node metadata
			s.ctx.Metrics().PopulateScoreMetaData()

			// Set fields based on if we found an allocation option
			if option != nil {
				resources := &structs.AllocatedResources{
					Tasks: option.TaskResources,
					Shared: structs.AllocatedSharedResources{
						DiskMB: int64(tg.EphemeralDisk.SizeMB),
					},
				}

				// Create an allocation for this
				alloc := &structs.Allocation{
					ID:                 uuid.Generate(),
					Namespace:          s.job.Namespace,
					EvalID:             s.eval.ID,
					Name:               missing.Name(),
					JobID:              s.job.ID,
					TaskGroup:          tg.Name,
					Metrics:            s.ctx.Metrics(),
					NodeID:             option.Node.ID,
					DeploymentID:       deploymentID,
					TaskResources:      resources.OldTaskResources(),
					AllocatedResources: resources,
					DesiredStatus:      structs.AllocDesiredStatusRun,
					ClientStatus:       structs.AllocClientStatusPending,

					SharedResources: &structs.Resources{
						DiskMB: tg.EphemeralDisk.SizeMB,
					},
				}

				// If the new allocation is replacing an older allocation then we
				// set the record the older allocation id so that they are chained
				if prevAllocation != nil {
					alloc.PreviousAllocation = prevAllocation.ID
					if missing.IsRescheduling() {
						updateRescheduleTracker(alloc, prevAllocation, now)
					}
				}

				// If we are placing a canary and we found a match, add the canary
				// to the deployment state object and mark it as a canary.
				if missing.Canary() && s.deployment != nil {
					if state, ok := s.deployment.TaskGroups[tg.Name]; ok {
						state.PlacedCanaries = append(state.PlacedCanaries, alloc.ID)
					}

					alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
						Canary: true,
					}
				}

				// Track the placement
				s.plan.AppendAlloc(alloc)

			} else {
				// Lazy initialize the failed map
				if s.failedTGAllocs == nil {
					s.failedTGAllocs = make(map[string]*structs.AllocMetric)
				}

				// Track the fact that we didn't find a placement
				s.failedTGAllocs[tg.Name] = s.ctx.Metrics()

				// If we weren't able to find a replacement for the allocation, back
				// out the fact that we asked to stop the allocation.
				if stopPrevAlloc {
					s.plan.PopUpdate(prevAllocation)
				}
			}

		}
	}

	return nil
}

// getSelectOptions sets up preferred nodes and penalty nodes
func getSelectOptions(prevAllocation *structs.Allocation, preferredNode *structs.Node) *SelectOptions {
	selectOptions := &SelectOptions{}
	if prevAllocation != nil {
		penaltyNodes := make(map[string]struct{})
		penaltyNodes[prevAllocation.NodeID] = struct{}{}
		if prevAllocation.RescheduleTracker != nil {
			for _, reschedEvent := range prevAllocation.RescheduleTracker.Events {
				penaltyNodes[reschedEvent.PrevNodeID] = struct{}{}
			}
		}
		selectOptions.PenaltyNodeIDs = penaltyNodes
	}
	if preferredNode != nil {
		selectOptions.PreferredNodes = []*structs.Node{preferredNode}
	}
	return selectOptions
}

// updateRescheduleTracker carries over previous restart attempts and adds the most recent restart
func updateRescheduleTracker(alloc *structs.Allocation, prev *structs.Allocation, now time.Time) {
	reschedPolicy := prev.ReschedulePolicy()
	var rescheduleEvents []*structs.RescheduleEvent
	if prev.RescheduleTracker != nil {
		var interval time.Duration
		if reschedPolicy != nil {
			interval = reschedPolicy.Interval
		}
		// If attempts is set copy all events in the interval range
		if reschedPolicy.Attempts > 0 {
			for _, reschedEvent := range prev.RescheduleTracker.Events {
				timeDiff := now.UnixNano() - reschedEvent.RescheduleTime
				// Only copy over events that are within restart interval
				// This keeps the list of events small in cases where there's a long chain of old restart events
				if interval > 0 && timeDiff <= interval.Nanoseconds() {
					rescheduleEvents = append(rescheduleEvents, reschedEvent.Copy())
				}
			}
		} else {
			// Only copy the last n if unlimited is set
			start := 0
			if len(prev.RescheduleTracker.Events) > maxPastRescheduleEvents {
				start = len(prev.RescheduleTracker.Events) - maxPastRescheduleEvents
			}
			for i := start; i < len(prev.RescheduleTracker.Events); i++ {
				reschedEvent := prev.RescheduleTracker.Events[i]
				rescheduleEvents = append(rescheduleEvents, reschedEvent.Copy())
			}
		}
	}
	nextDelay := prev.NextDelay()
	rescheduleEvent := structs.NewRescheduleEvent(now.UnixNano(), prev.ID, prev.NodeID, nextDelay)
	rescheduleEvents = append(rescheduleEvents, rescheduleEvent)
	alloc.RescheduleTracker = &structs.RescheduleTracker{Events: rescheduleEvents}
}

// findPreferredNode finds the preferred node for an allocation
func (s *GenericScheduler) findPreferredNode(place placementResult) (*structs.Node, error) {
	if prev := place.PreviousAllocation(); prev != nil && place.TaskGroup().EphemeralDisk.Sticky == true {
		var preferredNode *structs.Node
		ws := memdb.NewWatchSet()
		preferredNode, err := s.state.NodeByID(ws, prev.NodeID)
		if err != nil {
			return nil, err
		}

		if preferredNode != nil && preferredNode.Ready() {
			return preferredNode, nil
		}
	}
	return nil, nil
}
