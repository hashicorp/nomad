// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"runtime/debug"
	"slices"
	"sort"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler/feasible"
	"github.com/hashicorp/nomad/scheduler/reconciler"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

const (
	// maxServiceScheduleAttempts is used to limit the number of times
	// we will attempt to schedule if we continue to hit conflicts for services.
	maxServiceScheduleAttempts = 5

	// maxBatchScheduleAttempts is used to limit the number of times
	// we will attempt to schedule if we continue to hit conflicts for batch.
	maxBatchScheduleAttempts = 2

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
	logger   log.Logger
	eventsCh chan<- interface{}
	state    sstructs.State
	planner  sstructs.Planner
	batch    bool

	eval       *structs.Evaluation
	job        *structs.Job
	plan       *structs.Plan
	planResult *structs.PlanResult
	ctx        *feasible.EvalContext
	stack      *feasible.GenericStack

	// followUpEvals are evals with WaitUntil set, which are delayed until that time
	// before being rescheduled
	followUpEvals []*structs.Evaluation

	deployment *structs.Deployment

	blocked         *structs.Evaluation
	failedTGAllocs  map[string]*structs.AllocMetric
	queuedAllocs    map[string]int
	planAnnotations *structs.PlanAnnotations
}

// NewServiceScheduler is a factory function to instantiate a new service scheduler
func NewServiceScheduler(logger log.Logger, eventsCh chan<- interface{}, state sstructs.State, planner sstructs.Planner) sstructs.Scheduler {
	s := &GenericScheduler{
		logger:   logger.Named("service_sched"),
		eventsCh: eventsCh,
		state:    state,
		planner:  planner,
		batch:    false,
	}
	return s
}

// NewBatchScheduler is a factory function to instantiate a new batch scheduler
func NewBatchScheduler(logger log.Logger, eventsCh chan<- interface{}, state sstructs.State, planner sstructs.Planner) sstructs.Scheduler {
	s := &GenericScheduler{
		logger:   logger.Named("batch_sched"),
		eventsCh: eventsCh,
		state:    state,
		planner:  planner,
		batch:    true,
	}
	return s
}

// Process is used to handle a single evaluation
func (s *GenericScheduler) Process(eval *structs.Evaluation) (err error) {

	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("processing eval panicked scheduler - please report this as a bug!", "eval_id", eval.ID, "error", r, "stack_trace", string(debug.Stack()))
			err = fmt.Errorf("failed to process eval: %v", r)
		}
	}()

	// Store the evaluation
	s.eval = eval

	// Update our logger with the eval's information
	s.logger = s.logger.With("eval_id", eval.ID, "job_id", eval.JobID, "namespace", eval.Namespace)

	// Verify the evaluation trigger reason is understood
	switch eval.TriggeredBy {
	case structs.EvalTriggerJobRegister, structs.EvalTriggerJobDeregister,
		structs.EvalTriggerNodeDrain, structs.EvalTriggerNodeUpdate,
		structs.EvalTriggerAllocStop, structs.EvalTriggerAllocReschedule,
		structs.EvalTriggerRollingUpdate, structs.EvalTriggerQueuedAllocs,
		structs.EvalTriggerPeriodicJob, structs.EvalTriggerMaxPlans,
		structs.EvalTriggerDeploymentWatcher, structs.EvalTriggerRetryFailedAlloc,
		structs.EvalTriggerFailedFollowUp, structs.EvalTriggerPreemption,
		structs.EvalTriggerScaling, structs.EvalTriggerMaxDisconnectTimeout, structs.EvalTriggerReconnect:
	default:
		desc := fmt.Sprintf("scheduler cannot handle '%s' evaluation reason",
			eval.TriggeredBy)
		return setStatus(s.logger, s.planner, s.eval, nil, s.blocked,
			s.failedTGAllocs, s.planAnnotations, structs.EvalStatusFailed, desc, s.queuedAllocs,
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
				s.failedTGAllocs, s.planAnnotations, statusErr.EvalStatus, err.Error(),
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
		s.failedTGAllocs, s.planAnnotations, structs.EvalStatusComplete, "", s.queuedAllocs,
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

	s.blocked = s.eval.CreateBlockedEval(classEligibility, escaped, e.QuotaLimitReached(), s.failedTGAllocs)
	if planFailure {
		s.blocked.TriggeredBy = structs.EvalTriggerMaxPlans
		s.blocked.StatusDescription = sstructs.DescBlockedEvalMaxPlan
	} else {
		s.blocked.StatusDescription = sstructs.DescBlockedEvalFailedPlacements
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
		s.deployment = s.deployment.Copy() // may mutate in reconciler
	}

	// Reset the failed allocations
	s.failedTGAllocs = nil

	// Create an evaluation context
	s.ctx = feasible.NewEvalContext(s.eventsCh, s.state, s.plan, s.logger)

	// Construct the placement stack
	s.stack = feasible.NewGenericStack(s.batch, s.ctx)
	if !s.job.Stopped() {
		s.setJob(s.job)
	}

	// Compute the target job allocations
	if err := s.computeJobAllocs(); err != nil {
		s.logger.Error("failed to compute job allocations", "error", err)
		return false, err
	}

	// If there are failed allocations, we need to create a blocked evaluation
	// to place the failed allocations when resources become available. If the
	// current evaluation is already a blocked eval, we reuse it. If not, submit
	// a new eval to the planner in createBlockedEval. If rescheduling should
	// be delayed, do that instead.
	if s.eval.Status != structs.EvalStatusBlocked &&
		len(s.failedTGAllocs) != 0 &&
		s.blocked == nil &&
		(len(s.followUpEvals) == 0 || time.Now().After(s.eval.WaitUntil)) {
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

	// Create follow up evals for any delayed reschedule eligible allocations, except in
	// the case that this evaluation was already delayed.
	if len(s.followUpEvals) > 0 && s.eval.WaitUntil.IsZero() {
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
	if s.eval.AnnotatePlan {
		s.plan.Annotations = s.planAnnotations
	}
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
	// nodes to lost, but only if the scheduler has already marked them
	updateNonTerminalAllocsToLost(s.plan, tainted, allocs)

	r := reconciler.NewAllocReconciler(s.logger,
		genericAllocUpdateFn(s.ctx, s.stack, s.eval.ID),
		reconciler.ReconcilerState{
			Job:               s.job,
			JobID:             s.eval.JobID,
			JobIsBatch:        s.batch,
			DeploymentCurrent: s.deployment,
			ExistingAllocs:    allocs,
			EvalID:            s.eval.ID,
			EvalPriority:      s.eval.Priority,
		},
		reconciler.ClusterState{
			TaintedNodes: tainted,
			Now:          time.Now().UTC(),
		})
	result := r.Compute()
	if s.logger.IsDebug() {
		s.logger.Debug("reconciled current state with desired state", result.Fields()...)
	}

	s.planAnnotations = &structs.PlanAnnotations{
		DesiredTGUpdates: result.DesiredTGUpdates,
	}

	// Add the deployment changes to the plan
	s.plan.Deployment = result.Deployment
	s.plan.DeploymentUpdates = result.DeploymentUpdates

	// Store all the follow up evaluations from rescheduled allocations
	if len(result.DesiredFollowupEvals) > 0 {
		for _, evals := range result.DesiredFollowupEvals {
			s.followUpEvals = append(s.followUpEvals, evals...)
		}
	}

	// Update the stored deployment
	if result.Deployment != nil {
		s.deployment = result.Deployment
	}

	// Handle the stop
	for _, stop := range result.Stop {
		s.plan.AppendStoppedAlloc(stop.Alloc, stop.StatusDescription, stop.ClientStatus, stop.FollowupEvalID)
	}

	// Handle disconnect updates
	for _, update := range result.DisconnectUpdates {
		s.plan.AppendUnknownAlloc(update)
	}

	// Handle reconnect updates.
	// Reconnected allocs have a new AllocState entry.
	for _, update := range result.ReconnectUpdates {
		s.ctx.Plan().AppendAlloc(update, nil)
	}

	// Handle the in-place updates
	for _, update := range result.InplaceUpdate {
		if update.DeploymentID != s.deployment.GetID() {
			update.DeploymentID = s.deployment.GetID()
			update.DeploymentStatus = nil
		}
		s.ctx.Plan().AppendAlloc(update, nil)
	}

	// Handle the annotation updates
	for _, update := range result.AttributeUpdates {
		s.ctx.Plan().AppendAlloc(update, nil)
	}

	// Nothing remaining to do if placement is not required
	if len(result.Place)+len(result.DestructiveUpdate) == 0 {
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

	// Compute the placements
	place := make([]reconciler.PlacementResult, 0, len(result.Place))
	for _, p := range result.Place {
		s.queuedAllocs[p.TaskGroup().Name] += 1
		place = append(place, p)
	}

	destructive := make([]reconciler.PlacementResult, 0, len(result.DestructiveUpdate))
	for _, p := range result.DestructiveUpdate {
		s.queuedAllocs[p.TaskGroup().Name] += 1
		destructive = append(destructive, p)
	}
	return s.computePlacements(destructive, place, result.TaskGroupAllocNameIndexes)
}

// downgradedJobForPlacement returns the previous stable version of the job for
// downgrading a placement for non-canaries
func (s *GenericScheduler) downgradedJobForPlacement(p reconciler.PlacementResult) (string, *structs.Job, error) {
	ns, jobID := s.job.Namespace, s.job.ID
	tgName := p.TaskGroup().Name

	// find deployments and use the latest promoted or canaried version
	deployments, err := s.state.DeploymentsByJobID(nil, ns, jobID, false)
	if err != nil {
		return "", nil, fmt.Errorf("failed to lookup job deployments: %v", err)
	}

	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].JobVersion > deployments[j].JobVersion
	})

	for _, d := range deployments {
		// It's unexpected to have a recent deployment that doesn't contain the TaskGroup; as all allocations
		// should be destroyed. In such cases, attempt to find the deployment for that TaskGroup and hopefully
		// we will kill it soon.  This is a defensive measure, have not seen it in practice
		//
		// Zero dstate.DesiredCanaries indicates that the TaskGroup allocates were updated in-place without using canaries.
		if dstate := d.TaskGroups[tgName]; dstate != nil && (dstate.Promoted || dstate.DesiredCanaries == 0) {
			job, err := s.state.JobByIDAndVersion(nil, ns, jobID, d.JobVersion)
			return d.ID, job, err
		}
	}

	// check if the non-promoted version is a job without update block. This version should be the latest "stable" version,
	// as all subsequent versions must be canaried deployments.  Otherwise, we would have found a deployment above,
	// or the alloc would have been replaced already by a newer non-deployment job.
	if job, err := s.state.JobByIDAndVersion(nil, ns, jobID, p.MinJobVersion()); err == nil && job != nil && job.Update.IsEmpty() {
		return "", job, err
	}

	return "", nil, nil
}

// computePlacements computes placements for allocations. It is given the set of
// destructive updates to place and the set of new placements to place.
func (s *GenericScheduler) computePlacements(
	destructive, place []reconciler.PlacementResult, nameIndex map[string]*reconciler.AllocNameIndex,
) error {

	// Get the base nodes
	nodes, byDC, err := s.setNodes(s.job)
	if err != nil {
		return err
	}

	var deploymentID string
	if s.deployment != nil && s.deployment.Active() {
		deploymentID = s.deployment.ID
	}

	// Capture current time to use as the start time for any rescheduled allocations
	now := time.Now()

	// Have to handle destructive changes first as we need to discount their
	// resources. To understand this imagine the resources were reduced and the
	// count was scaled up.
	for _, results := range [][]reconciler.PlacementResult{destructive, place} {
		for _, missing := range results {
			// Get the task group
			tg := missing.TaskGroup()

			// This is populated from the reconciler via the compute results,
			// therefore we cannot have an allocation belonging to a task group
			// that has not generated and been through allocation name index
			// tracking.
			taskGroupNameIndex := nameIndex[tg.Name]

			var downgradedJob *structs.Job

			if missing.DowngradeNonCanary() {
				jobDeploymentID, job, err := s.downgradedJobForPlacement(missing)
				if err != nil {
					return err
				}

				// Defensive check - if there is no appropriate deployment for this job, use the latest
				if job != nil && job.Version >= missing.MinJobVersion() && job.LookupTaskGroup(tg.Name) != nil {
					tg = job.LookupTaskGroup(tg.Name)
					downgradedJob = job
					deploymentID = jobDeploymentID
				} else {
					jobVersion := -1
					if job != nil {
						jobVersion = int(job.Version)
					}
					s.logger.Debug("failed to find appropriate job; using the latest", "expected_version", missing.MinJobVersion, "found_version", jobVersion)
				}
			}

			// Check if this task group has already failed
			if metric, ok := s.failedTGAllocs[tg.Name]; ok {
				metric.CoalescedFailures += 1
				metric.ExhaustResources(tg)
				continue
			}

			// Use downgraded job in scheduling stack to honor old job
			// resources, constraints, and node pool scheduler configuration.
			if downgradedJob != nil {
				s.setJob(downgradedJob)

				if needsToSetNodes(downgradedJob, s.job) {
					nodes, byDC, err = s.setNodes(downgradedJob)
					if err != nil {
						return err
					}
				}
			}

			// Find the preferred node
			preferredNode, err := s.findPreferredNode(missing)
			if err != nil {
				return err
			}

			// Check if we should stop the previous allocation upon successful
			// placement of the new alloc. This allow atomic placements/stops. We
			// stop the allocation before trying to place the new alloc because this
			// frees the resources currently used by the previous allocation.
			stopPrevAlloc, stopPrevAllocDesc := missing.StopPreviousAlloc()
			prevAllocation := missing.PreviousAllocation()
			if stopPrevAlloc {
				s.plan.AppendStoppedAlloc(prevAllocation, stopPrevAllocDesc, "", "")
			}

			// Compute penalty nodes for rescheduled allocs
			selectOptions := getSelectOptions(prevAllocation, preferredNode)
			selectOptions.AllocName = missing.Name()
			option := s.selectNextOption(tg, selectOptions)

			// Store the available nodes by datacenter
			s.ctx.Metrics().NodesAvailable = byDC
			s.ctx.Metrics().NodesInPool = len(nodes)
			s.ctx.Metrics().NodePool = s.job.NodePool

			// Compute top K scoring node metadata
			s.ctx.Metrics().PopulateScoreMetaData()

			// Restore stack job and nodes now that placement is done, to use
			// plan job version
			if downgradedJob != nil {
				s.setJob(s.job)

				if needsToSetNodes(downgradedJob, s.job) {
					nodes, byDC, err = s.setNodes(s.job)
					if err != nil {
						return err
					}
				}
			}

			// Set fields based on if we found an allocation option
			if option != nil {
				resources := &structs.AllocatedResources{
					Tasks:          option.TaskResources,
					TaskLifecycles: option.TaskLifecycles,
					Shared: structs.AllocatedSharedResources{
						DiskMB: int64(tg.EphemeralDisk.SizeMB),
					},
				}
				if option.AllocResources != nil {
					resources.Shared.Networks = option.AllocResources.Networks
					resources.Shared.Ports = option.AllocResources.Ports
				}

				// Pull the allocation name as a new variables, so we can alter
				// this as needed without making changes to the original
				// object.
				newAllocName := missing.Name()

				// Identify the index from the name, so we can check this
				// against the allocation name index tracking for duplicates.
				allocIndex := structs.AllocIndexFromName(newAllocName, s.job.ID, tg.Name)

				// If the allocation index is a duplicate, we cannot simply
				// create a new allocation with the same name. We need to
				// generate a new index and use this. The log message is useful
				// for debugging and development, but could be removed in a
				// future version of Nomad.
				if taskGroupNameIndex.IsDuplicate(allocIndex) {
					oldAllocName := newAllocName
					newAllocName = taskGroupNameIndex.Next(1)[0]
					taskGroupNameIndex.UnsetIndex(allocIndex)
					s.logger.Debug("duplicate alloc index found and changed",
						"old_alloc_name", oldAllocName, "new_alloc_name", newAllocName)
				}

				// Create an allocation for this
				alloc := &structs.Allocation{
					ID:                 uuid.Generate(),
					Namespace:          s.job.Namespace,
					EvalID:             s.eval.ID,
					Name:               newAllocName,
					JobID:              s.job.ID,
					TaskGroup:          tg.Name,
					Metrics:            s.ctx.Metrics(),
					NodeID:             option.Node.ID,
					NodeName:           option.Node.Name,
					DeploymentID:       deploymentID,
					TaskResources:      resources.OldTaskResources(),
					AllocatedResources: resources,
					DesiredStatus:      structs.AllocDesiredStatusRun,
					ClientStatus:       structs.AllocClientStatusPending,
					// SharedResources is considered deprecated, will be removed in 0.11.
					// It is only set for compat reasons.
					SharedResources: &structs.Resources{
						DiskMB:   tg.EphemeralDisk.SizeMB,
						Networks: resources.Shared.Networks,
					},
				}

				// If the new allocation is replacing an older allocation then we
				// set the record the older allocation id so that they are chained
				if prevAllocation != nil {
					alloc.PreviousAllocation = prevAllocation.ID
					if missing.IsRescheduling() {
						original := prevAllocation
						prevAllocation = prevAllocation.Copy()
						missing.SetPreviousAllocation(prevAllocation)
						UpdateRescheduleTracker(alloc, prevAllocation, now)
						swapAllocInPlan(s.plan, original, prevAllocation)
					}
				}

				// If we are placing a canary and we found a match, add the canary
				// to the deployment state object and mark it as a canary.
				if missing.Canary() && s.deployment != nil {
					alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
						Canary: true,
					}
				}

				s.handlePreemptions(option, alloc, missing)

				// Track the placement
				s.plan.AppendAlloc(alloc, downgradedJob)

			} else {
				// Lazy initialize the failed map
				if s.failedTGAllocs == nil {
					s.failedTGAllocs = make(map[string]*structs.AllocMetric)
				}

				// Update metrics with the resources requested by the task group.
				s.ctx.Metrics().ExhaustResources(tg)

				// Track the fact that we didn't find a placement
				s.failedTGAllocs[tg.Name] = s.ctx.Metrics()

				// If we weren't able to find a placement for the allocation, back
				// out the fact that we asked to stop the allocation.
				if stopPrevAlloc {
					s.plan.PopUpdate(prevAllocation)
				}

				// If we were trying to replace a rescheduling alloc, mark the
				// reschedule as failed so that we can retry it in the following
				// blocked eval without dropping the reschedule tracker
				if prevAllocation != nil {
					if missing.IsRescheduling() {
						missing.SetPreviousAllocation(prevAllocation)
						markFailedToReschedule(s.plan, prevAllocation, s.job)
					}
				}

			}

		}
	}

	return nil
}

// markFailedToReschedule takes a "previous" allocation that we were unable to
// reschedule and updates the plan to annotate its reschedule tracker and to
// move it out of the stop list and into the update list so that we don't drop
// tracking information in the plan applier
func markFailedToReschedule(plan *structs.Plan, original *structs.Allocation, job *structs.Job) {
	updated := original.Copy()
	annotateRescheduleTracker(updated, structs.LastRescheduleFailedToPlace)

	plan.PopUpdate(original)
	nodeID := original.NodeID
	for i, alloc := range plan.NodeAllocation[nodeID] {
		if alloc.ID == original.ID {
			plan.NodeAllocation[nodeID][i] = updated
			return
		}
	}
	plan.AppendAlloc(updated, job)
}

// swapAllocInPlan updates a plan to swap out an allocation that's already in
// the plan with an updated definition of that allocation. The updated
// definition should be a deep copy.
func swapAllocInPlan(plan *structs.Plan, original, updated *structs.Allocation) {
	for i, stoppingAlloc := range plan.NodeUpdate[original.NodeID] {
		if stoppingAlloc.ID == original.ID {
			plan.NodeUpdate[original.NodeID][i] = updated
			return
		}
	}
	for i, alloc := range plan.NodeAllocation[original.NodeID] {
		if alloc.ID == original.ID {
			plan.NodeAllocation[original.NodeID][i] = updated
			return
		}
	}
}

// setJob updates the stack with the given job and job's node pool scheduler
// configuration.
func (s *GenericScheduler) setJob(job *structs.Job) error {
	// Fetch node pool and global scheduler configuration to determine how to
	// configure the scheduler.
	pool, err := s.state.NodePoolByName(nil, job.NodePool)
	if err != nil {
		return fmt.Errorf("failed to get job node pool %q: %v", job.NodePool, err)
	}

	_, schedConfig, err := s.state.SchedulerConfig()
	if err != nil {
		return fmt.Errorf("failed to get scheduler configuration: %v", err)
	}

	s.stack.SetJob(job)
	s.stack.SetSchedulerConfiguration(schedConfig.WithNodePool(pool))
	return nil
}

// setnodes updates the stack with the nodes that are ready for placement for
// the given job.
func (s *GenericScheduler) setNodes(job *structs.Job) ([]*structs.Node, map[string]int, error) {
	nodes, _, byDC, err := readyNodesInDCsAndPool(s.state, job.Datacenters, job.NodePool)
	if err != nil {
		return nil, nil, err
	}

	s.stack.SetNodes(nodes)
	return nodes, byDC, nil
}

// needsToSetNodes returns true if jobs a and b changed in a way that requires
// the nodes to be reset.
func needsToSetNodes(a, b *structs.Job) bool {
	return !helper.SliceSetEq(a.Datacenters, b.Datacenters) ||
		a.NodePool != b.NodePool
}

// getSelectOptions sets up preferred nodes and penalty nodes
func getSelectOptions(prevAllocation *structs.Allocation, preferredNode *structs.Node) *feasible.SelectOptions {
	selectOptions := &feasible.SelectOptions{}
	if prevAllocation != nil {
		penaltyNodes := make(map[string]struct{})

		// If alloc failed, penalize the node it failed on to encourage
		// rescheduling on a new node.
		if prevAllocation.ClientStatus == structs.AllocClientStatusFailed {
			penaltyNodes[prevAllocation.NodeID] = struct{}{}
		}
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

// annotateRescheduleTracker adds a note about the last reschedule attempt. This
// mutates the allocation, which should be a copy.
func annotateRescheduleTracker(prev *structs.Allocation, note structs.RescheduleTrackerAnnotation) {
	if prev.RescheduleTracker == nil {
		prev.RescheduleTracker = &structs.RescheduleTracker{}
	}
	prev.RescheduleTracker.LastReschedule = note
}

// UpdateRescheduleTracker carries over previous restart attempts and adds the
// most recent restart. This mutates both allocations; "alloc" is a new
// allocation so this is safe, but "prev" is coming from the state store and
// must be copied first.
func UpdateRescheduleTracker(alloc *structs.Allocation, prev *structs.Allocation, now time.Time) {
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
	alloc.RescheduleTracker = &structs.RescheduleTracker{
		Events:         rescheduleEvents,
		LastReschedule: structs.LastRescheduleSuccess}
	annotateRescheduleTracker(prev, structs.LastRescheduleSuccess)
}

// findPreferredNode finds the preferred node for an allocation
func (s *GenericScheduler) findPreferredNode(place reconciler.PlacementResult) (*structs.Node, error) {
	prev := place.PreviousAllocation()
	if prev == nil {
		return nil, nil
	}

	// when a jobs nodepool or datacenter are updated, we should ignore setting a preferred node
	// even if a task has ephemeral disk, as this would bypass the normal nodepool/datacenter node
	// selection logic, which would result in the alloc being place incorrectly.
	if prev.Job != nil && prev.Job.NodePool != s.job.NodePool {
		return nil, nil
	}
	if !slices.Equal(prev.Job.Datacenters, s.job.Datacenters) {
		return nil, nil
	}

	if place.TaskGroup().EphemeralDisk.Sticky || place.TaskGroup().EphemeralDisk.Migrate {
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

// selectNextOption calls the stack to get a node for placement
func (s *GenericScheduler) selectNextOption(tg *structs.TaskGroup, selectOptions *feasible.SelectOptions) *feasible.RankedNode {
	option := s.stack.Select(tg, selectOptions)
	_, schedConfig, _ := s.ctx.State().SchedulerConfig()

	// Check if preemption is enabled, defaults to true
	//
	// The scheduler configuration is read directly from state but only
	// values that can't be specified per node pool should be used. Other
	// values must be merged by calling schedConfig.WithNodePool() and set in
	// the stack by calling SetSchedulerConfiguration().
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

// handlePreemptions sets relevant preeemption related fields.
func (s *GenericScheduler) handlePreemptions(option *feasible.RankedNode, alloc *structs.Allocation, missing reconciler.PlacementResult) {
	if option.PreemptedAllocs == nil {
		return
	}

	// If this placement involves preemption, set DesiredState to evict for those allocations
	var preemptedAllocIDs []string
	for _, stop := range option.PreemptedAllocs {
		s.plan.AppendPreemptedAlloc(stop, alloc.ID)
		preemptedAllocIDs = append(preemptedAllocIDs, stop.ID)

		if s.planAnnotations != nil {
			s.planAnnotations.PreemptedAllocs = append(s.planAnnotations.PreemptedAllocs, stop.Stub(nil))
			if s.planAnnotations.DesiredTGUpdates != nil {
				desired := s.planAnnotations.DesiredTGUpdates[missing.TaskGroup().Name]
				desired.Preemptions += 1
			}
		}
	}

	alloc.PreemptedAllocations = preemptedAllocIDs
}
