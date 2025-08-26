// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"runtime/debug"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler/feasible"
	"github.com/hashicorp/nomad/scheduler/reconciler"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

const (
	// maxSystemScheduleAttempts is used to limit the number of times
	// we will attempt to schedule if we continue to hit conflicts for system
	// jobs.
	maxSystemScheduleAttempts = 5

	// maxSysBatchScheduleAttempts is used to limit the number of times we will
	// attempt to schedule if we continue to hit conflicts for sysbatch jobs.
	maxSysBatchScheduleAttempts = 2
)

// SystemScheduler is used for 'system' and 'sysbatch' jobs. This scheduler is
// designed for jobs that should be run on every client. The 'system' mode
// will ensure those jobs continuously run regardless of successful task exits,
// whereas 'sysbatch' considers the task complete on success.
type SystemScheduler struct {
	logger   log.Logger
	eventsCh chan<- interface{}
	state    sstructs.State
	planner  sstructs.Planner
	sysbatch bool

	eval       *structs.Evaluation
	job        *structs.Job
	plan       *structs.Plan
	planResult *structs.PlanResult
	ctx        *feasible.EvalContext
	stack      *feasible.SystemStack

	nodes         []*structs.Node
	notReadyNodes map[string]struct{}
	nodesByDC     map[string]int

	deployment *structs.Deployment

	limitReached bool
	nextEval     *structs.Evaluation

	failedTGAllocs  map[string]*structs.AllocMetric
	queuedAllocs    map[string]int
	planAnnotations *structs.PlanAnnotations
}

// NewSystemScheduler is a factory function to instantiate a new system
// scheduler.
func NewSystemScheduler(logger log.Logger, eventsCh chan<- interface{}, state sstructs.State, planner sstructs.Planner) sstructs.Scheduler {
	return &SystemScheduler{
		logger:   logger.Named("system_sched"),
		eventsCh: eventsCh,
		state:    state,
		planner:  planner,
		sysbatch: false,
	}
}

func NewSysBatchScheduler(logger log.Logger, eventsCh chan<- interface{}, state sstructs.State, planner sstructs.Planner) sstructs.Scheduler {
	return &SystemScheduler{
		logger:   logger.Named("sysbatch_sched"),
		eventsCh: eventsCh,
		state:    state,
		planner:  planner,
		sysbatch: true,
	}
}

// Process is used to handle a single evaluation.
func (s *SystemScheduler) Process(eval *structs.Evaluation) (err error) {

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
	if !s.canHandle(eval.TriggeredBy) {
		desc := fmt.Sprintf("scheduler cannot handle '%s' evaluation reason", eval.TriggeredBy)
		return setStatus(s.logger, s.planner, s.eval, s.nextEval, nil,
			s.failedTGAllocs, s.planAnnotations, structs.EvalStatusFailed, desc,
			s.queuedAllocs, "")
	}

	limit := maxSystemScheduleAttempts
	if s.sysbatch {
		limit = maxSysBatchScheduleAttempts
	}

	// Retry up to the maxSystemScheduleAttempts and reset if progress is made.
	progress := func() bool { return progressMade(s.planResult) }
	if err := retryMax(limit, s.process, progress); err != nil {
		if statusErr, ok := err.(*SetStatusError); ok {
			return setStatus(s.logger, s.planner, s.eval, s.nextEval, nil,
				s.failedTGAllocs, s.planAnnotations, statusErr.EvalStatus, err.Error(),
				s.queuedAllocs, "")
		}
		return err
	}

	// Update the status to complete
	return setStatus(s.logger, s.planner, s.eval, s.nextEval, nil,
		s.failedTGAllocs, s.planAnnotations, structs.EvalStatusComplete, "",
		s.queuedAllocs, "")
}

// process is wrapped in retryMax to iteratively run the handler until we have no
// further work or we've made the maximum number of attempts.
func (s *SystemScheduler) process() (bool, error) {
	// Lookup the Job by ID
	var err error
	ws := memdb.NewWatchSet()
	s.job, err = s.state.JobByID(ws, s.eval.Namespace, s.eval.JobID)
	if err != nil {
		return false, fmt.Errorf("failed to get job '%s': %v", s.eval.JobID, err)
	}

	numTaskGroups := 0
	if !s.job.Stopped() {
		numTaskGroups = len(s.job.TaskGroups)
	}
	s.queuedAllocs = make(map[string]int, numTaskGroups)

	// Get the ready nodes in the required datacenters
	if !s.job.Stopped() {
		s.nodes, s.notReadyNodes, s.nodesByDC, err = readyNodesInDCsAndPool(
			s.state, s.job.Datacenters, s.job.NodePool)
		if err != nil {
			return false, fmt.Errorf("failed to get ready nodes: %v", err)
		}
	}

	if !s.sysbatch {
		// Get any existing deployment
		s.deployment, err = s.state.LatestDeploymentByJobID(ws, s.eval.Namespace, s.eval.JobID)
		if err != nil {
			return false, fmt.Errorf("failed to get deployment for job %q: %w", s.eval.JobID, err)
		}
	}

	// Create a plan
	s.plan = s.eval.MakePlan(s.job)

	// Reset the failed allocations
	s.failedTGAllocs = nil

	// Create an evaluation context
	s.ctx = feasible.NewEvalContext(s.eventsCh, s.state, s.plan, s.logger)

	// Construct the placement stack
	s.stack = feasible.NewSystemStack(s.sysbatch, s.ctx)
	if !s.job.Stopped() {
		s.setJob(s.job)
	}

	// Compute the target job allocations
	if err := s.computeJobAllocs(); err != nil {
		s.logger.Error("failed to compute job allocations", "error", err)
		return false, err
	}

	// If the plan is a no-op, we can bail. If AnnotatePlan is set submit the plan
	// anyways to get the annotations.
	if s.plan.IsNoOp() && !s.eval.AnnotatePlan {
		return true, nil
	}

	// If the limit of placements was reached we need to create an evaluation
	// to pickup from here after the stagger period.
	if s.limitReached && s.nextEval == nil {
		s.nextEval = s.eval.NextRollingEval(s.job.Update.MinHealthyTime)
		if err := s.planner.CreateEval(s.nextEval); err != nil {
			s.logger.Error("failed to make next eval for rolling update", "error", err)
			return false, err
		}
		s.logger.Debug("rolling update limit reached, next eval created", "next_eval_id", s.nextEval.ID)
	}

	// Submit the plan
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
		return false, nil
	}

	// Success!
	return true, nil
}

// setJob updates the stack with the given job and job's node pool scheduler
// configuration.
func (s *SystemScheduler) setJob(job *structs.Job) error {
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

// computeJobAllocs is used to reconcile differences between the job,
// existing allocations and node status to update the allocations.
func (s *SystemScheduler) computeJobAllocs() error {
	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	allocs, err := s.state.AllocsByJob(ws, s.eval.Namespace, s.eval.JobID, true)
	if err != nil {
		return fmt.Errorf("failed to get allocs for job '%s': %v", s.eval.JobID, err)
	}

	// Determine the tainted nodes containing job allocs
	tainted, err := taintedNodes(s.state, allocs)
	if err != nil {
		return fmt.Errorf("failed to get tainted nodes for job '%s': %v", s.eval.JobID, err)
	}

	// Update the allocations which are in pending/running state on tainted
	// nodes to lost.
	updateNonTerminalAllocsToLost(s.plan, tainted, allocs)

	// Split out terminal allocations
	live, term := structs.SplitTerminalAllocs(allocs)

	// Diff the required and existing allocations
	nr := reconciler.NewNodeReconciler(s.deployment)
	r := nr.Compute(s.job, s.nodes, s.notReadyNodes, tainted, live, term,
		s.planner.ServersMeetMinimumVersion(minVersionMaxClientDisconnect, true))
	if s.logger.IsDebug() {
		s.logger.Debug("reconciled current state with desired state", r.Fields()...)
	}

	// Add the deployment changes to the plan
	s.plan.Deployment = nr.DeploymentCurrent
	s.plan.DeploymentUpdates = nr.DeploymentUpdates

	// Add all the allocs to stop
	for _, e := range r.Stop {
		s.plan.AppendStoppedAlloc(e.Alloc, sstructs.StatusAllocNotNeeded, "", "")
	}

	// Add all the allocs to migrate
	for _, e := range r.Migrate {
		s.plan.AppendStoppedAlloc(e.Alloc, sstructs.StatusAllocNodeTainted, "", "")
	}

	// Lost allocations should be transitioned to desired status stop and client
	// status lost.
	for _, e := range r.Lost {
		s.plan.AppendStoppedAlloc(e.Alloc, sstructs.StatusAllocLost, structs.AllocClientStatusLost, "")
	}

	for _, e := range r.Disconnecting {
		s.plan.AppendUnknownAlloc(e.Alloc)
	}

	allocExistsForTaskGroup := map[string]bool{}
	// Attempt to do the upgrades in place.
	// Reconnecting allocations need to be updated to persists alloc state
	// changes.
	updates := make([]reconciler.AllocTuple, 0, len(r.Update)+len(r.Reconnecting))
	updates = append(updates, r.Update...)
	updates = append(updates, r.Reconnecting...)
	destructiveUpdates, inplaceUpdates := inplaceUpdate(s.ctx, s.eval, s.job, s.stack, updates)
	r.Update = destructiveUpdates

	for _, inplaceUpdate := range inplaceUpdates {
		allocExistsForTaskGroup[inplaceUpdate.TaskGroup.Name] = true
	}

	s.planAnnotations = &structs.PlanAnnotations{
		DesiredTGUpdates: desiredUpdates(r, inplaceUpdates, destructiveUpdates),
	}

	// Check if a rolling upgrade strategy is being used
	limit := len(r.Update)
	if !s.job.Stopped() && s.job.Update.Rolling() {
		limit = s.job.Update.MaxParallel
	}

	// Treat non in-place updates as an eviction and new placement.
	s.limitReached = evictAndPlace(s.ctx, r, r.Update, sstructs.StatusAllocUpdating, &limit)

	// Nothing remaining to do if placement is not required
	if len(r.Place) == 0 {
		if !s.job.Stopped() {
			for _, tg := range s.job.TaskGroups {
				s.queuedAllocs[tg.Name] = 0
			}
		}
		return nil
	}

	// Record the number of allocations that needs to be placed per Task Group
	for _, allocTuple := range r.Place {
		s.queuedAllocs[allocTuple.TaskGroup.Name] += 1
	}

	for _, ignoredAlloc := range r.Ignore {
		allocExistsForTaskGroup[ignoredAlloc.TaskGroup.Name] = true
	}

	// Compute the placements
	return s.computePlacements(r.Place, allocExistsForTaskGroup)
}

func mergeNodeFiltered(acc, curr *structs.AllocMetric) *structs.AllocMetric {
	if acc == nil {
		return curr.Copy()
	}

	acc.NodesEvaluated += curr.NodesEvaluated
	acc.NodesFiltered += curr.NodesFiltered

	if acc.ClassFiltered == nil {
		acc.ClassFiltered = make(map[string]int)
	}
	for k, v := range curr.ClassFiltered {
		acc.ClassFiltered[k] += v
	}
	if acc.ConstraintFiltered == nil {
		acc.ConstraintFiltered = make(map[string]int)
	}
	for k, v := range curr.ConstraintFiltered {
		acc.ConstraintFiltered[k] += v
	}
	acc.AllocationTime += curr.AllocationTime
	return acc
}

// computePlacements computes placements for allocations
func (s *SystemScheduler) computePlacements(place []reconciler.AllocTuple, existingByTaskGroup map[string]bool) error {
	nodeByID := make(map[string]*structs.Node, len(s.nodes))
	for _, node := range s.nodes {
		nodeByID[node.ID] = node
	}

	// track node filtering, to only report an error if all nodes have been filtered
	var filteredMetrics map[string]*structs.AllocMetric

	var deploymentID string
	if s.deployment != nil && s.deployment.Active() {
		deploymentID = s.deployment.ID
	}

	nodes := make([]*structs.Node, 1)
	for _, missing := range place {
		tgName := missing.TaskGroup.Name

		node, ok := nodeByID[missing.Alloc.NodeID]
		if !ok {
			s.logger.Debug("could not find node", "node", missing.Alloc.NodeID)
			continue
		}

		// Update the set of placement nodes
		nodes[0] = node
		s.stack.SetNodes(nodes)

		// Attempt to match the task group
		option := s.stack.Select(missing.TaskGroup, &feasible.SelectOptions{AllocName: missing.Name})

		if option == nil {
			// If the task can't be placed on this node, update reporting data
			// and continue to short circuit the loop

			// If this node was filtered because of constraint
			// mismatches and we couldn't create an allocation then
			// decrement queuedAllocs for that task group.
			if s.ctx.Metrics().NodesFiltered > 0 {
				queued := s.queuedAllocs[tgName] - 1
				s.queuedAllocs[tgName] = queued

				if filteredMetrics == nil {
					filteredMetrics = map[string]*structs.AllocMetric{}
				}
				filteredMetrics[tgName] = mergeNodeFiltered(filteredMetrics[tgName], s.ctx.Metrics())

				// If no tasks have been placed and there aren't any previously
				// existing (ignored or updated) tasks on the node, mark the alloc as failed to be placed
				// if queued <= 0 && !existingByTaskGroup[tgName] {
				if queued <= 0 && !existingByTaskGroup[tgName] {
					if s.failedTGAllocs == nil {
						s.failedTGAllocs = make(map[string]*structs.AllocMetric)
					}
					s.failedTGAllocs[tgName] = filteredMetrics[tgName]
				}

				// If we are annotating the plan, then decrement the desired
				// placements based on whether the node meets the constraints
				if s.planAnnotations != nil &&
					s.planAnnotations.DesiredTGUpdates != nil {
					desired := s.planAnnotations.DesiredTGUpdates[tgName]
					desired.Place -= 1
				}

				// Filtered nodes are not reported to users, just omitted from the job status
				continue
			}

			// Check if this task group has already failed, reported to the user as a count
			if metric, ok := s.failedTGAllocs[tgName]; ok {
				metric.CoalescedFailures += 1
				metric.ExhaustResources(missing.TaskGroup)
				continue
			}

			// Store the available nodes by datacenter
			s.ctx.Metrics().NodesAvailable = s.nodesByDC
			s.ctx.Metrics().NodesInPool = len(s.nodes)
			s.ctx.Metrics().NodePool = s.job.NodePool

			// Compute top K scoring node metadata
			s.ctx.Metrics().PopulateScoreMetaData()

			// Lazy initialize the failed map
			if s.failedTGAllocs == nil {
				s.failedTGAllocs = make(map[string]*structs.AllocMetric)
			}

			// Update metrics with the resources requested by the task group.
			s.ctx.Metrics().ExhaustResources(missing.TaskGroup)

			// Actual failure to start this task on this candidate node, report it individually
			s.failedTGAllocs[tgName] = s.ctx.Metrics()
			s.addBlocked(node)

			continue
		}

		// Store the available nodes by datacenter
		s.ctx.Metrics().NodesAvailable = s.nodesByDC
		s.ctx.Metrics().NodesInPool = len(s.nodes)

		// Compute top K scoring node metadata
		s.ctx.Metrics().PopulateScoreMetaData()

		// Set fields based on if we found an allocation option
		resources := &structs.AllocatedResources{
			Tasks:          option.TaskResources,
			TaskLifecycles: option.TaskLifecycles,
			Shared: structs.AllocatedSharedResources{
				DiskMB: int64(missing.TaskGroup.EphemeralDisk.SizeMB),
			},
		}

		if option.AllocResources != nil {
			resources.Shared.Networks = option.AllocResources.Networks
			resources.Shared.Ports = option.AllocResources.Ports
		}

		// Create an allocation for this
		alloc := &structs.Allocation{
			ID:                 uuid.Generate(),
			Namespace:          s.job.Namespace,
			EvalID:             s.eval.ID,
			Name:               missing.Name,
			JobID:              s.job.ID,
			TaskGroup:          tgName,
			Metrics:            s.ctx.Metrics(),
			NodeID:             option.Node.ID,
			NodeName:           option.Node.Name,
			DeploymentID:       deploymentID,
			TaskResources:      resources.OldTaskResources(),
			AllocatedResources: resources,
			DesiredStatus:      structs.AllocDesiredStatusRun,
			ClientStatus:       structs.AllocClientStatusPending,
			// SharedResources is considered deprecated, will be removed in 0.11.
			// It is only set for compat reasons
			SharedResources: &structs.Resources{
				DiskMB:   missing.TaskGroup.EphemeralDisk.SizeMB,
				Networks: resources.Shared.Networks,
			},
		}

		// If the new allocation is replacing an older allocation then we record the
		// older allocation id so that they are chained
		if missing.Alloc != nil {
			alloc.PreviousAllocation = missing.Alloc.ID
		}

		// If this placement involves preemption, set DesiredState to evict for those allocations
		if option.PreemptedAllocs != nil {
			var preemptedAllocIDs []string
			for _, stop := range option.PreemptedAllocs {
				s.plan.AppendPreemptedAlloc(stop, alloc.ID)

				preemptedAllocIDs = append(preemptedAllocIDs, stop.ID)
				if s.eval.AnnotatePlan && s.planAnnotations != nil {
					s.planAnnotations.PreemptedAllocs = append(s.planAnnotations.PreemptedAllocs, stop.Stub(nil))
					if s.planAnnotations.DesiredTGUpdates != nil {
						desired := s.planAnnotations.DesiredTGUpdates[tgName]
						desired.Preemptions += 1
					}
				}
			}
			alloc.PreemptedAllocations = preemptedAllocIDs
		}

		s.plan.AppendAlloc(alloc, nil)
	}

	return nil
}

// addBlocked creates a new blocked eval for this job on this node
// and submit to the planner (worker.go), which keeps the eval for execution later
func (s *SystemScheduler) addBlocked(node *structs.Node) error {
	e := s.ctx.Eligibility()
	escaped := e.HasEscaped()

	// Only store the eligible classes if the eval hasn't escaped.
	var classEligibility map[string]bool
	if !escaped {
		classEligibility = e.GetClasses()
	}

	blocked := s.eval.CreateBlockedEval(classEligibility, escaped, e.QuotaLimitReached(), s.failedTGAllocs)
	blocked.StatusDescription = sstructs.DescBlockedEvalFailedPlacements
	blocked.NodeID = node.ID

	return s.planner.CreateEval(blocked)
}

func (s *SystemScheduler) canHandle(trigger string) bool {
	switch trigger {
	case structs.EvalTriggerJobRegister:
	case structs.EvalTriggerNodeUpdate:
	case structs.EvalTriggerFailedFollowUp:
	case structs.EvalTriggerJobDeregister:
	case structs.EvalTriggerRollingUpdate:
	case structs.EvalTriggerPreemption:
	case structs.EvalTriggerDeploymentWatcher:
	case structs.EvalTriggerNodeDrain:
	case structs.EvalTriggerAllocStop:
	case structs.EvalTriggerQueuedAllocs:
	case structs.EvalTriggerScaling:
	case structs.EvalTriggerReconnect:
	default:
		switch s.sysbatch {
		case true:
			return trigger == structs.EvalTriggerPeriodicJob
		case false:
			return false
		}
	}
	return true
}

// evictAndPlace is used to mark allocations for evicts and add them to the
// placement queue. evictAndPlace modifies both the diffResult and the
// limit. It returns true if the limit has been reached.
func evictAndPlace(ctx feasible.Context, diff *reconciler.NodeReconcileResult, allocs []reconciler.AllocTuple, desc string, limit *int) bool {
	n := len(allocs)
	for i := 0; i < n && i < *limit; i++ {
		a := allocs[i]
		ctx.Plan().AppendStoppedAlloc(a.Alloc, desc, "", "")
		diff.Place = append(diff.Place, a)
	}
	if n <= *limit {
		*limit -= n
		return false
	}
	*limit = 0
	return true
}
