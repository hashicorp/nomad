package nomad

import (
	"fmt"
	"runtime"
	"time"

	"github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
)

// planner is used to mange the submitted allocation plans that are waiting
// to be accessed by the leader
type planner struct {
	*Server
	log log.Logger

	// planQueue is used to manage the submitted allocation
	// plans that are waiting to be assessed by the leader
	planQueue *PlanQueue
}

// newPlanner returns a new planner to be used for managing allocation plans.
func newPlanner(s *Server) (*planner, error) {
	// Create a plan queue
	planQueue, err := NewPlanQueue()
	if err != nil {
		return nil, err
	}

	return &planner{
		Server:    s,
		log:       s.logger.Named("planner"),
		planQueue: planQueue,
	}, nil
}

// planApply is a long lived goroutine that reads plan allocations from
// the plan queue, determines if they can be applied safely and applies
// them via Raft.
//
// Naively, we could simply dequeue a plan, verify, apply and then respond.
// However, the plan application is bounded by the Raft apply time and
// subject to some latency. This creates a stall condition, where we are
// not evaluating, but simply waiting for a transaction to apply.
//
// To avoid this, we overlap verification with apply. This means once
// we've verified plan N we attempt to apply it. However, while waiting
// for apply, we begin to verify plan N+1 under the assumption that plan
// N has succeeded.
//
// In this sense, we track two parallel versions of the world. One is
// the pessimistic one driven by the Raft log which is replicated. The
// other is optimistic and assumes our transactions will succeed. In the
// happy path, this lets us do productive work during the latency of
// apply.
//
// In the unhappy path (Raft transaction fails), effectively we only
// wasted work during a time we would have been waiting anyways. However,
// in anticipation of this case we cannot respond to the plan until
// the Raft log is updated. This means our schedulers will stall,
// but there are many of those and only a single plan verifier.
//
func (p *planner) planApply() {
	// waitCh is used to track an outstanding application while snap
	// holds an optimistic state which includes that plan application.
	var waitCh chan struct{}
	var snap *state.StateSnapshot

	// Setup a worker pool with half the cores, with at least 1
	poolSize := runtime.NumCPU() / 2
	if poolSize == 0 {
		poolSize = 1
	}
	pool := NewEvaluatePool(poolSize, workerPoolBufferSize)
	defer pool.Shutdown()

	for {
		// Pull the next pending plan, exit if we are no longer leader
		pending, err := p.planQueue.Dequeue(0)
		if err != nil {
			return
		}

		// Check if out last plan has completed
		select {
		case <-waitCh:
			waitCh = nil
			snap = nil
		default:
		}

		// Snapshot the state so that we have a consistent view of the world
		// if no snapshot is available
		if waitCh == nil || snap == nil {
			snap, err = p.fsm.State().Snapshot()
			if err != nil {
				p.logger.Error("failed to snapshot state", "error", err)
				pending.respond(nil, err)
				continue
			}
		}

		// Evaluate the plan
		result, err := evaluatePlan(pool, snap, pending.plan, p.logger)
		if err != nil {
			p.logger.Error("failed to evaluate plan", "error", err)
			pending.respond(nil, err)
			continue
		}

		// Fast-path the response if there is nothing to do
		if result.IsNoOp() {
			pending.respond(result, nil)
			continue
		}

		// Ensure any parallel apply is complete before starting the next one.
		// This also limits how out of date our snapshot can be.
		if waitCh != nil {
			<-waitCh
			snap, err = p.fsm.State().Snapshot()
			if err != nil {
				p.logger.Error("failed to snapshot state", "error", err)
				pending.respond(nil, err)
				continue
			}
		}

		// Dispatch the Raft transaction for the plan
		future, err := p.applyPlan(pending.plan, result, snap)
		if err != nil {
			p.logger.Error("failed to submit plan", "error", err)
			pending.respond(nil, err)
			continue
		}

		// Respond to the plan in async
		waitCh = make(chan struct{})
		go p.asyncPlanWait(waitCh, future, result, pending)
	}
}

// applyPlan is used to apply the plan result and to return the alloc index
func (p *planner) applyPlan(plan *structs.Plan, result *structs.PlanResult, snap *state.StateSnapshot) (raft.ApplyFuture, error) {
	// Determine the minimum number of updates, could be more if there
	// are multiple updates per node
	minUpdates := len(result.NodeUpdate)
	minUpdates += len(result.NodeAllocation)

	// Setup the update request
	req := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Job:   plan.Job,
			Alloc: make([]*structs.Allocation, 0, minUpdates),
		},
		Deployment:        result.Deployment,
		DeploymentUpdates: result.DeploymentUpdates,
		EvalID:            plan.EvalID,
		NodePreemptions:   make([]*structs.Allocation, 0, len(result.NodePreemptions)),
	}
	for _, updateList := range result.NodeUpdate {
		req.Alloc = append(req.Alloc, updateList...)
	}
	for _, allocList := range result.NodeAllocation {
		req.Alloc = append(req.Alloc, allocList...)
	}

	for _, preemptions := range result.NodePreemptions {
		req.NodePreemptions = append(req.NodePreemptions, preemptions...)
	}

	// Set the time the alloc was applied for the first time. This can be used
	// to approximate the scheduling time.
	now := time.Now().UTC().UnixNano()
	for _, alloc := range req.Alloc {
		if alloc.CreateTime == 0 {
			alloc.CreateTime = now
		}
		alloc.ModifyTime = now
	}

	// Set modify time for preempted allocs if any
	// Also gather jobids to create follow up evals
	preemptedJobIDs := make(map[structs.NamespacedID]struct{})
	for _, alloc := range req.NodePreemptions {
		alloc.ModifyTime = now
		id := structs.NamespacedID{Namespace: alloc.Namespace, ID: alloc.JobID}
		_, ok := preemptedJobIDs[id]
		if !ok {
			preemptedJobIDs[id] = struct{}{}
		}
	}

	var evals []*structs.Evaluation
	for preemptedJobID := range preemptedJobIDs {
		job, _ := p.State().JobByID(nil, preemptedJobID.Namespace, preemptedJobID.ID)
		if job != nil {
			eval := &structs.Evaluation{
				ID:          uuid.Generate(),
				Namespace:   job.Namespace,
				TriggeredBy: structs.EvalTriggerPreemption,
				JobID:       job.ID,
				Type:        job.Type,
				Priority:    job.Priority,
				Status:      structs.EvalStatusPending,
			}
			evals = append(evals, eval)
		}
	}
	req.PreemptionEvals = evals

	// Dispatch the Raft transaction
	future, err := p.raftApplyFuture(structs.ApplyPlanResultsRequestType, &req)
	if err != nil {
		return nil, err
	}

	// Optimistically apply to our state view
	if snap != nil {
		nextIdx := p.raft.AppliedIndex() + 1
		if err := snap.UpsertPlanResults(nextIdx, &req); err != nil {
			return future, err
		}
	}
	return future, nil
}

// asyncPlanWait is used to apply and respond to a plan async
func (p *planner) asyncPlanWait(waitCh chan struct{}, future raft.ApplyFuture,
	result *structs.PlanResult, pending *pendingPlan) {
	defer metrics.MeasureSince([]string{"nomad", "plan", "apply"}, time.Now())
	defer close(waitCh)

	// Wait for the plan to apply
	if err := future.Error(); err != nil {
		p.logger.Error("failed to apply plan", "error", err)
		pending.respond(nil, err)
		return
	}

	// Respond to the plan
	result.AllocIndex = future.Index()

	// If this is a partial plan application, we need to ensure the scheduler
	// at least has visibility into any placements it made to avoid double placement.
	// The RefreshIndex computed by evaluatePlan may be stale due to evaluation
	// against an optimistic copy of the state.
	if result.RefreshIndex != 0 {
		result.RefreshIndex = maxUint64(result.RefreshIndex, result.AllocIndex)
	}
	pending.respond(result, nil)
}

// evaluatePlan is used to determine what portions of a plan
// can be applied if any. Returns if there should be a plan application
// which may be partial or if there was an error
func evaluatePlan(pool *EvaluatePool, snap *state.StateSnapshot, plan *structs.Plan, logger log.Logger) (*structs.PlanResult, error) {
	defer metrics.MeasureSince([]string{"nomad", "plan", "evaluate"}, time.Now())

	// Check if the plan exceeds quota
	overQuota, err := evaluatePlanQuota(snap, plan)
	if err != nil {
		return nil, err
	}

	// Reject the plan and force the scheduler to refresh
	if overQuota {
		index, err := refreshIndex(snap)
		if err != nil {
			return nil, err
		}

		logger.Debug("plan for evaluation exceeds quota limit. Forcing state refresh", "eval_id", plan.EvalID, "refresh_index", index)
		return &structs.PlanResult{RefreshIndex: index}, nil
	}

	return evaluatePlanPlacements(pool, snap, plan, logger)
}

// evaluatePlanPlacements is used to determine what portions of a plan can be
// applied if any, looking for node over commitment. Returns if there should be
// a plan application which may be partial or if there was an error
func evaluatePlanPlacements(pool *EvaluatePool, snap *state.StateSnapshot, plan *structs.Plan, logger log.Logger) (*structs.PlanResult, error) {
	// Create a result holder for the plan
	result := &structs.PlanResult{
		NodeUpdate:        make(map[string][]*structs.Allocation),
		NodeAllocation:    make(map[string][]*structs.Allocation),
		Deployment:        plan.Deployment.Copy(),
		DeploymentUpdates: plan.DeploymentUpdates,
		NodePreemptions:   make(map[string][]*structs.Allocation),
	}

	// Collect all the nodeIDs
	nodeIDs := make(map[string]struct{})
	nodeIDList := make([]string, 0, len(plan.NodeUpdate)+len(plan.NodeAllocation))
	for nodeID := range plan.NodeUpdate {
		if _, ok := nodeIDs[nodeID]; !ok {
			nodeIDs[nodeID] = struct{}{}
			nodeIDList = append(nodeIDList, nodeID)
		}
	}
	for nodeID := range plan.NodeAllocation {
		if _, ok := nodeIDs[nodeID]; !ok {
			nodeIDs[nodeID] = struct{}{}
			nodeIDList = append(nodeIDList, nodeID)
		}
	}

	// Setup a multierror to handle potentially getting many
	// errors since we are processing in parallel.
	var mErr multierror.Error
	partialCommit := false

	// handleResult is used to process the result of evaluateNodePlan
	handleResult := func(nodeID string, fit bool, reason string, err error) (cancel bool) {
		// Evaluate the plan for this node
		if err != nil {
			mErr.Errors = append(mErr.Errors, err)
			return true
		}
		if !fit {
			// Log the reason why the node's allocations could not be made
			if reason != "" {
				logger.Debug("plan for node rejected", "node_id", nodeID, "reason", reason)
			}
			// Set that this is a partial commit
			partialCommit = true

			// If we require all-at-once scheduling, there is no point
			// to continue the evaluation, as we've already failed.
			if plan.AllAtOnce {
				result.NodeUpdate = nil
				result.NodeAllocation = nil
				result.DeploymentUpdates = nil
				result.Deployment = nil
				result.NodePreemptions = nil
				return true
			}

			// Skip this node, since it cannot be used.
			return
		}

		// Add this to the plan result
		if nodeUpdate := plan.NodeUpdate[nodeID]; len(nodeUpdate) > 0 {
			result.NodeUpdate[nodeID] = nodeUpdate
		}
		if nodeAlloc := plan.NodeAllocation[nodeID]; len(nodeAlloc) > 0 {
			result.NodeAllocation[nodeID] = nodeAlloc
		}

		if nodePreemptions := plan.NodePreemptions[nodeID]; nodePreemptions != nil {

			// Do a pass over preempted allocs in the plan to check
			// whether the alloc is already in a terminal state
			var filteredNodePreemptions []*structs.Allocation
			for _, preemptedAlloc := range nodePreemptions {
				alloc, err := snap.AllocByID(nil, preemptedAlloc.ID)
				if err != nil {
					mErr.Errors = append(mErr.Errors, err)
					continue
				}
				if alloc != nil && !alloc.TerminalStatus() {
					filteredNodePreemptions = append(filteredNodePreemptions, preemptedAlloc)
				}
			}

			result.NodePreemptions[nodeID] = filteredNodePreemptions
		}

		return
	}

	// Get the pool channels
	req := pool.RequestCh()
	resp := pool.ResultCh()
	outstanding := 0
	didCancel := false

	// Evaluate each node in the plan, handling results as they are ready to
	// avoid blocking.
OUTER:
	for len(nodeIDList) > 0 {
		nodeID := nodeIDList[0]
		select {
		case req <- evaluateRequest{snap, plan, nodeID}:
			outstanding++
			nodeIDList = nodeIDList[1:]
		case r := <-resp:
			outstanding--

			// Handle a result that allows us to cancel evaluation,
			// which may save time processing additional entries.
			if cancel := handleResult(r.nodeID, r.fit, r.reason, r.err); cancel {
				didCancel = true
				break OUTER
			}
		}
	}

	// Drain the remaining results
	for outstanding > 0 {
		r := <-resp
		if !didCancel {
			if cancel := handleResult(r.nodeID, r.fit, r.reason, r.err); cancel {
				didCancel = true
			}
		}
		outstanding--
	}

	// If the plan resulted in a partial commit, we need to determine
	// a minimum refresh index to force the scheduler to work on a more
	// up-to-date state to avoid the failures.
	if partialCommit {
		index, err := refreshIndex(snap)
		if err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
		result.RefreshIndex = index

		if result.RefreshIndex == 0 {
			err := fmt.Errorf("partialCommit with RefreshIndex of 0")
			mErr.Errors = append(mErr.Errors, err)
		}

		// If there was a partial commit and we are operating within a
		// deployment correct for any canary that may have been desired to be
		// placed but wasn't actually placed
		correctDeploymentCanaries(result)
	}
	return result, mErr.ErrorOrNil()
}

// correctDeploymentCanaries ensures that the deployment object doesn't list any
// canaries as placed if they didn't actually get placed. This could happen if
// the plan had a partial commit.
func correctDeploymentCanaries(result *structs.PlanResult) {
	// Hot path
	if result.Deployment == nil || !result.Deployment.HasPlacedCanaries() {
		return
	}

	// Build a set of all the allocations IDs that were placed
	placedAllocs := make(map[string]struct{}, len(result.NodeAllocation))
	for _, placed := range result.NodeAllocation {
		for _, alloc := range placed {
			placedAllocs[alloc.ID] = struct{}{}
		}
	}

	// Go through all the canaries and ensure that the result list only contains
	// those that have been placed
	for _, group := range result.Deployment.TaskGroups {
		canaries := group.PlacedCanaries
		if len(canaries) == 0 {
			continue
		}

		// Prune the canaries in place to avoid allocating an extra slice
		i := 0
		for _, canaryID := range canaries {
			if _, ok := placedAllocs[canaryID]; ok {
				canaries[i] = canaryID
				i++
			}
		}

		group.PlacedCanaries = canaries[:i]
	}
}

// evaluateNodePlan is used to evaluate the plan for a single node,
// returning if the plan is valid or if an error is encountered
func evaluateNodePlan(snap *state.StateSnapshot, plan *structs.Plan, nodeID string) (bool, string, error) {
	// If this is an evict-only plan, it always 'fits' since we are removing things.
	if len(plan.NodeAllocation[nodeID]) == 0 {
		return true, "", nil
	}

	// Get the node itself
	ws := memdb.NewWatchSet()
	node, err := snap.NodeByID(ws, nodeID)
	if err != nil {
		return false, "", fmt.Errorf("failed to get node '%s': %v", nodeID, err)
	}

	// If the node does not exist or is not ready for scheduling it is not fit
	// XXX: There is a potential race between when we do this check and when
	// the Raft commit happens.
	if node == nil {
		return false, "node does not exist", nil
	} else if node.Status != structs.NodeStatusReady {
		return false, "node is not ready for placements", nil
	} else if node.SchedulingEligibility == structs.NodeSchedulingIneligible {
		return false, "node is not eligible for draining", nil
	} else if node.Drain {
		// Deprecate in favor of scheduling eligibility and remove post-0.8
		return false, "node is draining", nil
	}

	// Get the existing allocations that are non-terminal
	existingAlloc, err := snap.AllocsByNodeTerminal(ws, nodeID, false)
	if err != nil {
		return false, "", fmt.Errorf("failed to get existing allocations for '%s': %v", nodeID, err)
	}

	// Determine the proposed allocation by first removing allocations
	// that are planned evictions and adding the new allocations.
	var remove []*structs.Allocation
	if update := plan.NodeUpdate[nodeID]; len(update) > 0 {
		remove = append(remove, update...)
	}

	// Remove any preempted allocs
	if preempted := plan.NodePreemptions[nodeID]; len(preempted) > 0 {
		for _, allocs := range preempted {
			remove = append(remove, allocs)
		}
	}

	if updated := plan.NodeAllocation[nodeID]; len(updated) > 0 {
		for _, alloc := range updated {
			remove = append(remove, alloc)
		}
	}
	proposed := structs.RemoveAllocs(existingAlloc, remove)
	proposed = append(proposed, plan.NodeAllocation[nodeID]...)

	// Check if these allocations fit
	fit, reason, _, err := structs.AllocsFit(node, proposed, nil, true)
	return fit, reason, err
}
