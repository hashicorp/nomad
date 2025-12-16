// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"maps"
	"math"
	"runtime/debug"
	"slices"

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
)

// SystemScheduler is used for 'system' jobs. This scheduler is designed for
// jobs that should be run on every client. The 'system' mode will ensure those
// jobs continuously run regardless of successful task exits, whereas 'sysbatch'
// considers the task complete on success.
type SystemScheduler struct {
	logger   log.Logger
	eventsCh chan<- interface{}
	state    sstructs.State
	planner  sstructs.Planner

	eval       *structs.Evaluation
	job        *structs.Job
	plan       *structs.Plan
	planResult *structs.PlanResult
	ctx        *feasible.EvalContext
	stack      *feasible.SystemStack

	nodes         []*structs.Node
	notReadyNodes map[string]struct{}
	nodesByDC     map[string]int

	deployment      *structs.Deployment
	failedTGAllocs  map[string]*structs.AllocMetric
	queuedAllocs    map[string]int
	planAnnotations *structs.PlanAnnotations

	tgCandidateNodeCounts     map[string]int // count of candidate nodes for the task group
	tgDestructiveUpdateCounts map[string]int // count of destructive updates for the task group
	tgExistingCanaryCount     map[string]int // count of currently running canaries for the task group
}

// NewSystemScheduler is a factory function to instantiate a new system
// scheduler.
func NewSystemScheduler(logger log.Logger, eventsCh chan<- interface{}, state sstructs.State, planner sstructs.Planner) sstructs.Scheduler {
	return &SystemScheduler{
		logger:   logger.Named("system_sched"),
		eventsCh: eventsCh,
		state:    state,
		planner:  planner,
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
		return setStatus(s.logger, s.planner, s.eval, nil, nil,
			s.failedTGAllocs, s.planAnnotations, structs.EvalStatusFailed, desc,
			s.queuedAllocs, s.deployment.GetID())
	}

	limit := maxSystemScheduleAttempts

	// Retry up to the maxSystemScheduleAttempts and reset if progress is made.
	progress := func() bool { return progressMade(s.planResult) }
	if err := retryMax(limit, s.process, progress); err != nil {
		if statusErr, ok := err.(*SetStatusError); ok {
			return setStatus(s.logger, s.planner, s.eval, nil, nil,
				s.failedTGAllocs, s.planAnnotations, statusErr.EvalStatus, err.Error(),
				s.queuedAllocs, s.deployment.GetID())
		}
		return err
	}

	// Update the status to complete
	return setStatus(s.logger, s.planner, s.eval, nil, nil,
		s.failedTGAllocs, s.planAnnotations, structs.EvalStatusComplete, "",
		s.queuedAllocs, s.deployment.GetID())
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

	s.deployment, err = s.state.LatestDeploymentByJobID(ws, s.eval.Namespace, s.eval.JobID)
	if err != nil {
		return false, fmt.Errorf("failed to get deployment for job %q: %w", s.eval.JobID, err)
	}
	// system deployments may be mutated in the reconciler because the node
	// count can change between evaluations
	s.deployment = s.deployment.Copy()

	// Create a plan
	s.plan = s.eval.MakePlan(s.job)

	// Reset the failed allocations
	s.failedTGAllocs = nil

	// Create an evaluation context
	s.ctx = feasible.NewEvalContext(s.eventsCh, s.state, s.plan, s.logger)

	// Construct the placement stack
	s.stack = feasible.NewSystemStack(false, s.ctx)
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
	reconciliationResult := nr.Compute(s.job, s.nodes, s.notReadyNodes, tainted,
		live, term)
	if s.logger.IsDebug() {
		s.logger.Debug("reconciled current state with desired state", reconciliationResult.Fields()...)
	}

	// Update the stored deployment
	if nr.DeploymentCurrent != nil {
		s.deployment = nr.DeploymentCurrent
	}

	// Add all the allocs to stop
	for _, e := range reconciliationResult.Stop {
		s.plan.AppendStoppedAlloc(e.Alloc, sstructs.StatusAllocNotNeeded, "", "")
	}

	// Add all the allocs to migrate
	for _, e := range reconciliationResult.Migrate {
		s.plan.AppendStoppedAlloc(e.Alloc, sstructs.StatusAllocNodeTainted, "", "")
	}

	// Lost allocations should be transitioned to desired status stop and client
	// status lost.
	for _, e := range reconciliationResult.Lost {
		s.plan.AppendStoppedAlloc(e.Alloc, sstructs.StatusAllocLost, structs.AllocClientStatusLost, "")
	}

	for _, e := range reconciliationResult.Disconnecting {
		s.plan.AppendUnknownAlloc(e.Alloc)
	}

	allocExistsForTaskGroup := map[string]bool{}
	// Attempt to do the upgrades in place.
	// Reconnecting allocations need to be updated to persists alloc state
	// changes.
	updates := make([]reconciler.AllocTuple, 0, len(reconciliationResult.Update)+len(reconciliationResult.Reconnecting))
	updates = append(updates, reconciliationResult.Update...)
	updates = append(updates, reconciliationResult.Reconnecting...)
	destructiveUpdates, inplaceUpdates := inplaceUpdate(s.ctx, s.eval, s.job, s.stack, updates, s.deployment.GetID())
	reconciliationResult.Update = destructiveUpdates

	s.planAnnotations = &structs.PlanAnnotations{
		DesiredTGUpdates: desiredUpdates(reconciliationResult, inplaceUpdates, destructiveUpdates),
	}

	// Initialize all the counts to track and generate
	// the node mapping so we can use it throughout.
	// Then loop through the various buckets to set
	// our initial counts.
	s.tgCandidateNodeCounts = make(map[string]int)
	s.tgExistingCanaryCount = make(map[string]int)
	s.tgDestructiveUpdateCounts = make(map[string]int)
	nodeByID := make(map[string]*structs.Node, len(s.nodes))
	for _, node := range s.nodes {
		nodeByID[node.ID] = node
	}

	// Add every allocation within the place bucket to the
	// candidate node count
	for _, allocTuple := range reconciliationResult.Place {
		s.tgCandidateNodeCounts[allocTuple.TaskGroup.Name]++
	}

	// Add every allocation that was updated in place to the
	// candidate node count. Also add them to the inplace
	// update count so they can be included in the deployment's
	// healthy alloc count.
	for _, allocTuple := range inplaceUpdates {
		s.tgCandidateNodeCounts[allocTuple.TaskGroup.Name]++
		allocExistsForTaskGroup[allocTuple.TaskGroup.Name] = true
	}

	// Add every allcation within the ignore bucket to the
	// candidate node count. Find any canaries within the
	// bucket and add them to the existing canary count so
	// we can properly calculate required canaries.
	for _, allocTuple := range reconciliationResult.Ignore {
		s.tgCandidateNodeCounts[allocTuple.TaskGroup.Name]++
		if allocTuple.Alloc.DeploymentStatus != nil && allocTuple.Alloc.DeploymentStatus.Canary {
			s.tgExistingCanaryCount[allocTuple.TaskGroup.Name]++
		}
	}

	// Add every allcation within the update bucket to the
	// candidate node count. The allocations within the
	// update bucket are destructive updates, so add them
	// to the destructive update count.
	for _, allocTuple := range reconciliationResult.Update {
		s.tgCandidateNodeCounts[allocTuple.TaskGroup.Name]++
		s.tgDestructiveUpdateCounts[allocTuple.TaskGroup.Name]++
	}

	// Treat non in-place updates as an eviction and new placement, which will
	// be limited by max_parallel
	s.evictAndPlace(reconciliationResult, sstructs.StatusAllocUpdating)

	if !s.job.Stopped() {
		for _, tg := range s.job.TaskGroups {
			s.queuedAllocs[tg.Name] = 0
		}
	}

	// Record the number of allocations that needs to be placed per Task Group
	for _, allocTuple := range reconciliationResult.Place {
		s.queuedAllocs[allocTuple.TaskGroup.Name] += 1
	}

	// Record that allocs currently exist for a Task Group
	for _, ignoredAlloc := range reconciliationResult.Ignore {
		allocExistsForTaskGroup[ignoredAlloc.TaskGroup.Name] = true
	}

	// Compute the placements
	if err := s.computePlacements(reconciliationResult, nodeByID, allocExistsForTaskGroup); err != nil {
		return err
	}

	// Set the desired placements into the annotation using the computed counts.
	for tgName := range s.tgCandidateNodeCounts {
		s.planAnnotations.DesiredTGUpdates[tgName].Place = uint64(s.tgCandidateNodeCounts[tgName])
	}

	// if there is no deployment we're done at this point
	if s.deployment == nil {
		return nil
	}

	// we only know the total amount of placements once we filter out infeasible
	// nodes, so for system jobs we do it backwards a bit: the "desired" total
	// is the total we were able to place.
	// track if any of the task groups is doing a canary update now
	deploymentComplete := true
	for _, tg := range s.job.TaskGroups {
		candidateCount := s.tgCandidateNodeCounts[tg.Name]

		if candidateCount < 1 {
			// this will happen if we're seeing a TG that shouldn't be placed.
			//
			// in case the deployment is in a successful state, this indicate a
			// noop eval due to infeasible nodes. In this case we set the dstate
			// for this task group to nil.
			if s.deployment.Status == structs.DeploymentStatusSuccessful {
				s.deployment.TaskGroups[tg.Name] = nil
			}

			continue
		}

		dstate, ok := s.deployment.TaskGroups[tg.Name]
		// no deployment for this TG
		if !ok {
			continue
		}

		// a system job is canarying if:
		// - it has a non-empty update block (just a sanity check, all
		// submitted jobs should have a non-empty update block as part of
		// canonicalization)
		// - canary parameter in the update block has to be positive
		// - deployment has to be non-nil and it cannot have been promoted
		// - this cannot be the initial job version
		isCanarying := !tg.Update.IsEmpty() &&
			tg.Update.Canary > 0 &&
			dstate != nil &&
			!dstate.Promoted &&
			s.job.Version != 0 &&
			s.tgDestructiveUpdateCounts[tg.Name] > 0

		// we can set the desired total now
		s.deployment.TaskGroups[tg.Name].DesiredTotal = candidateCount

		if isCanarying {
			// we can now also set the desired canaries: it's the
			// tg.Update.Canary percent of allocations that will
			// be destructively updated, rounded up to the nearest
			// int capped by the max_parallel
			destructiveCount := s.tgDestructiveUpdateCounts[tg.Name] + s.tgExistingCanaryCount[tg.Name]
			requiredCanaries := int(math.Ceil(float64(tg.Update.Canary) * float64(destructiveCount) / 100))
			requiredCanaries = min(requiredCanaries, tg.Update.MaxParallel)

			s.deployment.TaskGroups[tg.Name].DesiredCanaries = requiredCanaries

			// Initially, if the job requires canaries, we place all of them on
			// all eligible nodes. At this point we know which nodes are
			// feasible, so we evict unnedded canaries.
			placedCanaries := s.evictUnneededCanaries(requiredCanaries, tg.Name, reconciliationResult)

			// Update deployment and plan annotation with canaries that were placed
			s.deployment.TaskGroups[tg.Name].PlacedCanaries = placedCanaries
			s.planAnnotations.DesiredTGUpdates[tg.Name].Canary = uint64(len(placedCanaries))
		}

		groupComplete := s.isDeploymentComplete(dstate, isCanarying)
		deploymentComplete = deploymentComplete && groupComplete
	}

	// adjust the deployment updates and set the right deployment status
	nr.DeploymentUpdates = append(nr.DeploymentUpdates, s.setDeploymentStatusAndUpdates(deploymentComplete, s.job)...)

	// Check if perhaps we're dealing with a nil deployment, i.e., a deployment
	// which is in successful state and where all task groups have a nil dstate.
	// In this case, set the deployment to nil.
	nilDstates := true
	for _, tg := range s.deployment.TaskGroups {
		if tg != nil {
			nilDstates = false
		}
	}
	if nilDstates {
		s.deployment = nil
		nr.DeploymentUpdates = nil
	}

	// Add the deployment changes to the plan
	s.plan.Deployment = s.deployment
	s.plan.DeploymentUpdates = nr.DeploymentUpdates

	return nil
}

// computePlacements computes placements for allocations
func (s *SystemScheduler) computePlacements(
	reconcilerResult *reconciler.NodeReconcileResult, nodeByID map[string]*structs.Node, existingByTaskGroup map[string]bool,
) error {

	var deploymentID string
	if s.deployment != nil && s.deployment.Active() {
		deploymentID = s.deployment.ID
	}

	filteredMetrics := map[string]*structs.AllocMetric{}
	nodes := make([]*structs.Node, 1)

	for _, missing := range reconcilerResult.Place {
		tgName := missing.TaskGroup.Name

		node, ok := nodeByID[missing.Alloc.NodeID]
		if !ok {
			s.logger.Debug("could not find node", "node", missing.Alloc.NodeID)
			continue
		}

		nodes[0] = node
		s.stack.SetNodes(nodes)

		option := s.stack.Select(missing.TaskGroup, &feasible.SelectOptions{AllocName: missing.Name})

		if option == nil {
			// If the task can't be placed on this node, update reporting data
			// and continue to short circuit the loop

			// Since we are here, the node was not feasible. Decrement
			// the candidate node counts for this task group.
			s.tgCandidateNodeCounts[tgName]--

			// If this was an allocation being updated and there are no feasible
			// nodes, then it does not count as a destructive update so decrement
			// the count.
			if missing.Alloc.ID != "" {
				s.tgDestructiveUpdateCounts[tgName]--
			}

			// If this node was filtered because of constraint
			// mismatches and we couldn't create an allocation then
			// decrement queuedAllocs for that task group.
			if s.ctx.Metrics().NodesFiltered > 0 {
				s.queuedAllocs[tgName]--
				filteredMetrics[tgName] = mergeNodeFiltered(filteredMetrics[tgName], s.ctx.Metrics())

				// If no tasks have been placed and there aren't any previously
				// existing (ignored or updated) tasks on the node, mark the alloc
				// as failed to be placed
				if s.queuedAllocs[tgName] <= 0 && !existingByTaskGroup[tgName] {
					if s.failedTGAllocs == nil {
						s.failedTGAllocs = make(map[string]*structs.AllocMetric)
					}
					s.failedTGAllocs[tgName] = filteredMetrics[tgName]
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

		// If we are placing a canary, add the canary to the deployment state
		// object and mark it as a canary.
		if missing.Canary && s.deployment != nil {
			alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
				Canary: true,
			}
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
		return false
	}
	return true
}

// evictAndPlace is used to mark allocations for evicts and add them to the
// placement queue. evictAndPlace modifies the reconciler result. It returns
// true if the limit has been reached for any task group.
func (s *SystemScheduler) evictAndPlace(reconciled *reconciler.NodeReconcileResult, desc string) {

	limits := map[string]int{} // per task group limits
	if !s.job.Stopped() {
		jobLimit := len(reconciled.Update)
		if s.job.Update.MaxParallel > 0 {
			jobLimit = s.job.Update.MaxParallel
		}
		for _, tg := range s.job.TaskGroups {
			if tg.Update != nil && tg.Update.MaxParallel > 0 {
				limits[tg.Name] = tg.Update.MaxParallel
			} else {
				limits[tg.Name] = jobLimit
			}
		}
	}

	for _, a := range reconciled.Update {
		if limit := limits[a.Alloc.TaskGroup]; limit > 0 {
			s.ctx.Plan().AppendStoppedAlloc(a.Alloc, desc, "", "")
			reconciled.Place = append(reconciled.Place, a)

			// canaries will get limited when we cancel unneeded canaries later on
			if !a.Canary {
				limits[a.Alloc.TaskGroup]--
			}
		}
	}

	// it may be the case that there are keys in the NodeUpdate that are empty.
	// We should delete them, otherwise the plan won't be correctly recognize as
	// a no-op.
	maps.DeleteFunc(s.plan.NodeUpdate, func(k string, v []*structs.Allocation) bool {
		return len(v) == 0
	})
}

// evictAndPlaceCanaries checks how many canaries are needed against the amount
// of feasible nodes, and removes unnecessary placements from the plan.
func (s *SystemScheduler) evictUnneededCanaries(requiredCanaries int, tgName string, buckets *reconciler.NodeReconcileResult) []string {

	desiredCanaries := make([]string, 0)

	// no canaries to consider, quit early
	if requiredCanaries == 0 {
		return desiredCanaries
	}

	canaryCounter := requiredCanaries

	// Start with finding any existing failed canaries
	failedCanaries := map[string]struct{}{}
	for _, alloc := range buckets.Place {
		if alloc.Alloc != nil && alloc.Alloc.DeploymentStatus != nil && alloc.Alloc.DeploymentStatus.Canary {
			failedCanaries[alloc.Alloc.ID] = struct{}{}
		}
	}

	// Generate a list of preferred allocations for
	// canaries. These are existing canary applications
	// that are failed.
	preferCanary := map[string]struct{}{}
	for _, allocations := range s.plan.NodeAllocation {
		for _, alloc := range allocations {
			if _, ok := failedCanaries[alloc.PreviousAllocation]; ok {
				preferCanary[alloc.ID] = struct{}{}
			}
		}
	}

	// Remove the number of preferred canaries found
	// from the counter.
	canaryCounter -= len(preferCanary)

	// Check for any canaries that are already running. For any
	// that are found, add to the desired list and decrement
	// the counter.
	for _, tuple := range buckets.Ignore {
		if tuple.TaskGroup.Name == tgName && tuple.Alloc != nil &&
			tuple.Alloc.DeploymentStatus != nil && tuple.Alloc.DeploymentStatus.Canary {
			desiredCanaries = append(desiredCanaries, tuple.Alloc.ID)
			canaryCounter--
		}
	}

	// iterate over node allocations to find canary allocs
	for node, allocations := range s.plan.NodeAllocation {
		n := 0
		for _, alloc := range allocations {
			// these are the allocs we keep
			if alloc.DeploymentStatus == nil || !alloc.DeploymentStatus.Canary || alloc.TaskGroup != tgName {
				allocations[n] = alloc
				n += 1
				continue
			}

			// if it's a canary, we only keep up to desiredCanaries amount of
			// them
			if alloc.DeploymentStatus.Canary {
				// Check that the canary is on an eligble node
				if _, ineligibleNode := s.notReadyNodes[alloc.NodeID]; ineligibleNode {
					continue
				}

				// Check if this is a preferred allocation for the canary
				_, preferred := preferCanary[alloc.ID]

				// If it is a preferred allocation, or the counter is not exhausted,
				// keep the allocation
				if canaryCounter > 0 || preferred {
					canaryCounter -= 1
					desiredCanaries = append(desiredCanaries, alloc.ID)
					allocations[n] = alloc
					n += 1
				} else {
					// If the counter has been exhausted the allocation will not be
					// placed, but a stop will have been appended for the update.
					// Locate it and remove it.
					idx := slices.IndexFunc(s.plan.NodeUpdate[alloc.NodeID], func(a *structs.Allocation) bool {
						return a.ID == alloc.PreviousAllocation
					})
					if idx > -1 {
						s.plan.NodeUpdate[alloc.NodeID] = append(s.plan.NodeUpdate[alloc.NodeID][0:idx], s.plan.NodeUpdate[alloc.NodeID][idx+1:]...)
					}
				}
			}
		}

		// because of this nifty trick we don't need to allocate an extra slice
		s.plan.NodeAllocation[node] = allocations[:n]
	}

	return desiredCanaries
}

func (s *SystemScheduler) isDeploymentComplete(dstate *structs.DeploymentState, isCanarying bool) bool {
	if s.deployment == nil || isCanarying {
		return false
	}

	complete := true

	// ensure everything is healthy
	if dstate.HealthyAllocs < dstate.DesiredTotal { // Make sure we have enough healthy allocs
		complete = false
	}

	return complete
}

func (s *SystemScheduler) setDeploymentStatusAndUpdates(deploymentComplete bool, job *structs.Job) []*structs.DeploymentStatusUpdate {
	statusUpdates := []*structs.DeploymentStatusUpdate{}

	if d := s.deployment; d != nil {

		// Deployments that require promotion should have appropriate status set
		// immediately, no matter their completness.
		if d.RequiresPromotion() {
			if d.HasAutoPromote() {
				d.StatusDescription = structs.DeploymentStatusDescriptionRunningAutoPromotion
			} else {
				d.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
			}
			return statusUpdates
		}

		// Mark the deployment as complete if possible
		if deploymentComplete {
			if job.IsMultiregion() {
				// the unblocking/successful states come after blocked, so we
				// need to make sure we don't revert those states
				if d.Status != structs.DeploymentStatusUnblocking &&
					d.Status != structs.DeploymentStatusSuccessful {
					statusUpdates = append(statusUpdates, &structs.DeploymentStatusUpdate{
						DeploymentID:      s.deployment.ID,
						Status:            structs.DeploymentStatusBlocked,
						StatusDescription: structs.DeploymentStatusDescriptionBlocked,
					})
				}
			} else {
				statusUpdates = append(statusUpdates, &structs.DeploymentStatusUpdate{
					DeploymentID:      s.deployment.ID,
					Status:            structs.DeploymentStatusSuccessful,
					StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
				})
			}
		}

		// Mark the deployment as pending since its state is now computed.
		if d.Status == structs.DeploymentStatusInitializing {
			statusUpdates = append(statusUpdates, &structs.DeploymentStatusUpdate{
				DeploymentID:      s.deployment.ID,
				Status:            structs.DeploymentStatusPending,
				StatusDescription: structs.DeploymentStatusDescriptionPendingForPeer,
			})
		}
	}

	return statusUpdates
}
