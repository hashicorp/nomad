package scheduler

import (
	"sort"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// allocReconciler is used to determine the set of allocations that require
// placement, inplace updating or stopping given the job specification and
// existing cluster state. The reconciler should only be used for batch and
// service jobs.
type allocReconciler struct {
	// ctx gives access to the state store and logger
	ctx Context

	// stack allows checking for the ability to do an in-place update
	stack Stack

	// batch marks whether the job is a batch job
	batch bool

	// eval is the evaluation triggering the scheduling event
	eval *structs.Evaluation

	// job is the job being operated on, it may be nil if the job is being
	// stopped via a purge
	job *structs.Job

	// deployment is the current deployment for the job
	deployment *structs.Deployment

	// deploymentPaused marks whether the deployment is paused
	deploymentPaused bool

	// taintedNodes contains a map of nodes that are tainted
	taintedNodes map[string]*structs.Node

	// existingAllocs is non-terminal existing allocations
	existingAllocs []*structs.Allocation

	// result is the results of the reconcile. During computation it can be
	// used to store intermediate state
	result *reconcileResults
}

// reconcileResults contains the results of the reconciliation and should be
// applied by the scheduler.
type reconcileResults struct {
	// createDeployment is the deployment that should be created as a result of
	// scheduling
	createDeployment *structs.Deployment

	// deploymentUpdates contains a set of deployment updates that should be
	// applied as a result of scheduling
	deploymentUpdates []*structs.DeploymentStatusUpdate

	// place is the set of allocations to place by the scheduler
	place []allocPlaceResult

	// inplaceUpdate is the set of allocations to apply an inplace update to
	inplaceUpdate []*structs.Allocation

	// stop is the set of allocations to stop
	stop []allocStopResult

	// TODO track the desired of the deployment
}

// allocPlaceResult contains the information required to place a single
// allocation
type allocPlaceResult struct {
	name          string
	canary        bool
	taskGroup     *structs.TaskGroup
	previousAlloc *structs.Allocation
}

// allocStopResult contains the information required to stop a single allocation
type allocStopResult struct {
	alloc             *structs.Allocation
	clientStatus      string
	statusDescription string
}

// NewAllocReconciler creates a new reconciler that should be used to determine
// the changes required to bring the cluster state inline with the declared jobspec
func NewAllocReconciler(ctx Context, stack Stack, batch bool,
	eval *structs.Evaluation, job *structs.Job, deployment *structs.Deployment,
	existingAllocs []*structs.Allocation, taintedNodes map[string]*structs.Node) *allocReconciler {

	a := &allocReconciler{
		ctx:            ctx,
		stack:          stack,
		eval:           eval,
		batch:          batch,
		job:            job,
		deployment:     deployment,
		existingAllocs: existingAllocs,
		taintedNodes:   taintedNodes,
		result:         new(reconcileResults),
	}

	// Detect if the deployment is paused
	if deployment != nil {
		a.deploymentPaused = deployment.Status == structs.DeploymentStatusPaused
	}

	return a
}

// Compute reconciles the existing cluster state and returns the set of changes
// required to converge the job spec and state
func (a *allocReconciler) Compute() *reconcileResults {
	// If we are just stopping a job we do not need to do anything more than
	// stopping all running allocs
	stopped := a.job == nil || a.job.Stop
	if stopped {
		a.handleStop()

		// Cancel the deployment since it is not needed
		if a.deployment != nil {
			a.result.deploymentUpdates = append(a.result.deploymentUpdates, &structs.DeploymentStatusUpdate{
				DeploymentID:      a.deployment.ID,
				Status:            structs.DeploymentStatusCancelled,
				StatusDescription: structs.DeploymentStatusDescriptionStoppedJob,
			})
		}

		return a.result
	}

	// Check if the deployment is referencing an older job and cancel it
	if d := a.deployment; d != nil {
		if d.JobCreateIndex != a.job.CreateIndex || d.JobModifyIndex != a.job.JobModifyIndex {
			a.result.deploymentUpdates = append(a.result.deploymentUpdates, &structs.DeploymentStatusUpdate{
				DeploymentID:      a.deployment.ID,
				Status:            structs.DeploymentStatusCancelled,
				StatusDescription: structs.DeploymentStatusDescriptionNewerJob,
			})
			a.deployment = nil
		}
	}

	// Create a new deployment if necessary
	if a.deployment == nil && !stopped && a.job.HasUpdateStrategy() {
		a.deployment = structs.NewDeployment(a.job)
		a.result.createDeployment = a.deployment
		a.ctx.Logger().Printf("ALEX: MADE DEPLOYMENT %q", a.deployment.ID)
	}

	if a.deployment != nil {
		a.ctx.Logger().Printf("ALEX: CURRENT DEPLOYMENT %q", a.deployment.ID)
	}

	m := newAllocMatrix(a.job, a.existingAllocs)
	for group, as := range m {
		a.computeGroup(group, as)
	}

	return a.result
}

// handleStop marks all allocations to be stopped, handling the lost case
func (a *allocReconciler) handleStop() {
	as := newAllocSet(a.existingAllocs)
	untainted, migrate, lost := as.filterByTainted(a.taintedNodes)
	a.markStop(untainted, "", allocNotNeeded)
	a.markStop(migrate, "", allocNotNeeded)
	a.markStop(lost, structs.AllocClientStatusLost, allocLost)
}

// markStop is a helper for marking a set of allocation for stop with a
// particular client status and description.
func (a *allocReconciler) markStop(allocs allocSet, clientStatus, statusDescription string) {
	for _, alloc := range allocs {
		a.result.stop = append(a.result.stop, allocStopResult{
			alloc:             alloc,
			clientStatus:      clientStatus,
			statusDescription: statusDescription,
		})
	}
}

// computeGroup reconciles state for a particular task group.
func (a *allocReconciler) computeGroup(group string, as allocSet) {
	// Get the task group. The task group may be nil if the job was updates such
	// that the task group no longer exists
	tg := a.job.LookupTaskGroup(group)

	// Determine what set of alloations are on tainted nodes
	untainted, migrate, lost := as.filterByTainted(a.taintedNodes)

	a.ctx.Logger().Printf("RECONCILER -- untainted (%d); migrate (%d); lost (%d)", len(untainted), len(migrate), len(lost))
	a.ctx.Logger().Printf("RECONCILER -- untainted %#v", untainted)

	// If the task group is nil, then the task group has been removed so all we
	// need to do is stop everything
	if tg == nil {
		a.ctx.Logger().Printf("RECONCILER -- STOPPING ALL")
		a.markStop(untainted, "", allocNotNeeded)
		a.markStop(migrate, "", allocNotNeeded)
		a.markStop(lost, structs.AllocClientStatusLost, allocLost)
		return
	}

	// Get the deployment state for the group
	var dstate *structs.DeploymentState
	if a.deployment != nil {
		dstate = a.deployment.TaskGroups[group]
	}

	// Mark all lost allocations for stop. Previous allocation doesn't matter
	// here since it is on a lost node
	for _, alloc := range lost {
		a.result.stop = append(a.result.stop, allocStopResult{
			alloc:             alloc,
			clientStatus:      structs.AllocClientStatusLost,
			statusDescription: allocLost,
		})
	}

	// Get any existing canaries
	canaries := untainted.filterByCanary()

	// Cancel any canary from a prior deployment
	if len(canaries) != 0 {
		if a.deployment != nil {
			current, older := canaries.filterByDeployment(a.deployment.ID)
			a.markStop(older, "", allocNotNeeded)

			a.ctx.Logger().Printf("RECONCILER -- older canaries %#v", older)
			a.ctx.Logger().Printf("RECONCILER -- current canaries %#v", current)

			untainted = untainted.difference(older, current)
			canaries = current
			a.ctx.Logger().Printf("RECONCILER -- untainted - remove canaries %#v", untainted)
		} else {
			// We don't need any of those canaries since there no longer is a
			// deployment
			a.markStop(canaries, "", allocNotNeeded)
			untainted = untainted.difference(canaries)
			canaries = nil
			a.ctx.Logger().Printf("RECONCILER -- untainted - remove canaries %#v", untainted)
		}
	}

	// Stop any unneeded allocations and update the untainted set to not
	// included stopped allocations
	keep, stop := a.computeStop(tg, untainted)
	a.markStop(stop, "", allocNotNeeded)
	untainted = keep

	// Do inplace upgrades where possible and capture the set of upgrades that
	// need to be done destructively.
	_, inplace, destructive := a.computeUpdates(tg, untainted)

	a.ctx.Logger().Printf("RECONCILER -- Stopping (%d); Untainted (%d)", len(stop), len(keep))
	a.ctx.Logger().Printf("RECONCILER -- Inplace (%d); Destructive (%d)", len(inplace), len(destructive))

	// Get the update strategy of the group
	strategy := tg.Update

	// XXX need a structure for picking names

	// The fact that we have destructive updates and have less canaries than is
	// desired means we need to create canaries
	requireCanary := len(destructive) != 0 && strategy != nil && len(canaries) < strategy.Canary
	placeCanaries := requireCanary && !a.deploymentPaused
	if placeCanaries {
		a.ctx.Logger().Printf("RECONCILER -- Canary (%d)", strategy.Canary-len(canaries))
		for i := len(canaries); i < strategy.Canary; i++ {
			a.result.place = append(a.result.place, allocPlaceResult{
				// XXX Pick better name
				name:      structs.GenerateUUID(),
				canary:    true,
				taskGroup: tg,
			})
		}
	}

	// Determine how many we can place
	haveCanaries := len(canaries) != 0 || placeCanaries
	limit := a.computeLimit(tg, untainted, haveCanaries)
	a.ctx.Logger().Printf("RECONCILER -- LIMIT %v", limit)

	// Place if:
	// * The deployment is not paused
	// * Not placing any canaries
	// * If there are any canaries that they have been promoted
	existingCanariesPromoted := dstate == nil || dstate.DesiredCanaries == 0 || dstate.Promoted
	canPlace := !a.deploymentPaused && !requireCanary && existingCanariesPromoted
	a.ctx.Logger().Printf("RECONCILER -- CAN PLACE %v", canPlace)
	if canPlace {
		// Place all new allocations
		place := a.computePlacements(tg, untainted)
		a.ctx.Logger().Printf("RECONCILER -- Placing (%d)", len(place))
		for _, p := range place {
			a.result.place = append(a.result.place, p)
		}

		// Do all destructive updates
		min := helper.IntMin(len(destructive), limit)
		i := 0
		a.ctx.Logger().Printf("RECONCILER -- Destructive Updating (%d)", min)
		for _, alloc := range destructive {
			if i == min {
				break
			}
			i++

			a.result.stop = append(a.result.stop, allocStopResult{
				alloc:             alloc,
				statusDescription: allocUpdating,
			})
			a.result.place = append(a.result.place, allocPlaceResult{
				name:          alloc.Name,
				taskGroup:     tg,
				previousAlloc: alloc,
			})
		}
		limit -= min
	}

	// TODO Migrations should be done using a stagger and max_parallel.
	a.ctx.Logger().Printf("RECONCILER -- Migrating (%d)", len(migrate))
	for _, alloc := range migrate {
		a.result.stop = append(a.result.stop, allocStopResult{
			alloc:             alloc,
			statusDescription: allocMigrating,
		})
		a.result.place = append(a.result.place, allocPlaceResult{
			name:          alloc.Name,
			canary:        false,
			taskGroup:     tg,
			previousAlloc: alloc,
		})
	}
}

// computeLimit returns the placement limit for a particular group. The inputs
// are the group definition, the existing/untainted allocation set and whether
// any canaries exist or are being placed.
func (a *allocReconciler) computeLimit(group *structs.TaskGroup, untainted allocSet, canaries bool) int {
	// If there is no update stategy or deployment for the group we can deploy
	// as many as the group has
	if group.Update == nil || a.deployment == nil {
		return group.Count
	} else if a.deploymentPaused {
		// If the deployment is paused, do not create anything else
		return 0
	}

	// Get the state of the deployment for the group
	deploymentState := a.deployment.TaskGroups[group.Name]

	// If we have canaries and they have not been promoted the limit is 0
	if canaries && (deploymentState == nil || !deploymentState.Promoted) {
		return 0
	}

	// If we have been promoted or there are no canaries, the limit is the
	// configured MaxParallel - any outstanding non-healthy alloc for the
	// deployment
	limit := group.Update.MaxParallel
	partOf, _ := untainted.filterByDeployment(a.deployment.ID)
	for _, alloc := range partOf {
		if alloc.DeploymentStatus == nil || alloc.DeploymentStatus.Healthy == nil {
			limit--
		}
	}

	return limit
}

// computePlacement returns the set of allocations to place given the group
// definiton and the set of untainted/existing allocations for the group.
func (a *allocReconciler) computePlacements(group *structs.TaskGroup, untainted allocSet) []allocPlaceResult {
	// Hot path the nothing to do case
	existing := len(untainted)
	if existing == group.Count {
		return nil
	}

	// XXX need to pick better names
	var place []allocPlaceResult
	for i := existing; i < group.Count; i++ {
		place = append(place, allocPlaceResult{
			name:      structs.GenerateUUID(),
			taskGroup: group,
		})
	}

	return place
}

// computeStop returns the set of allocations to stop given the group definiton
// and the set of untainted/existing allocations for the group.
func (a *allocReconciler) computeStop(group *structs.TaskGroup, untainted allocSet) (keep, stop allocSet) {
	// Hot path the nothing to do case
	if len(untainted) <= group.Count {
		return untainted, nil
	}

	// XXX Sort doesn't actually do the right thing "foo.bar[11]" < "foo.bar[3]"
	// TODO make name tree
	names := make([]string, 0, len(untainted))
	for name := range untainted {
		names = append(names, name)
	}
	sort.Strings(names)

	keep = make(map[string]*structs.Allocation)
	stop = make(map[string]*structs.Allocation)

	for i, name := range names {
		a := untainted[name]
		if i < group.Count {
			keep[a.Name] = a
		} else {
			stop[a.Name] = a
		}
	}

	return
}

// computeUpdates determines which allocations for the passed group require
// updates. Three groups are returned:
// 1. Those that require no upgrades
// 2. Those that can be upgraded in-place. These are added to the results
// automatically since the function contains the correct state to do so,
// 3. Those that require destructive updates
func (a *allocReconciler) computeUpdates(group *structs.TaskGroup, untainted allocSet) (ignore, inplace, destructive allocSet) {
	// Determine the set of allocations that need to be updated
	ignore = make(map[string]*structs.Allocation)
	inplace = make(map[string]*structs.Allocation)
	destructive = make(map[string]*structs.Allocation)

	ws := memdb.NewWatchSet()
	for _, alloc := range untainted {
		if alloc.Job.JobModifyIndex == a.job.JobModifyIndex {
			ignore[alloc.ID] = alloc
			continue
		}

		// Check if the task drivers or config has changed, requires
		// a destructive upgrade since that cannot be done in-place.
		if tasksUpdated(a.job, alloc.Job, group.Name) {
			destructive[alloc.ID] = alloc
			continue
		}

		// Terminal batch allocations are not filtered when they are completed
		// successfully. We should avoid adding the allocation to the plan in
		// the case that it is an in-place update to avoid both additional data
		// in the plan and work for the clients.
		if alloc.TerminalStatus() {
			ignore[alloc.ID] = alloc
			continue
		}

		// Get the existing node
		node, err := a.ctx.State().NodeByID(ws, alloc.NodeID)
		if err != nil {
			a.ctx.Logger().Printf("[ERR] sched: %#v failed to get node '%s': %v", a.eval, alloc.NodeID, err)
			continue
		}
		if node == nil {
			destructive[alloc.ID] = alloc
			continue
		}

		// Set the existing node as the base set
		a.stack.SetNodes([]*structs.Node{node})

		// Stage an eviction of the current allocation. This is done so that
		// the current allocation is discounted when checking for feasability.
		// Otherwise we would be trying to fit the tasks current resources and
		// updated resources. After select is called we can remove the evict.
		a.ctx.Plan().AppendUpdate(alloc, structs.AllocDesiredStatusStop, allocInPlace, "")

		// Attempt to match the task group
		option, _ := a.stack.Select(group)

		// Pop the allocation
		a.ctx.Plan().PopUpdate(alloc)

		// Skip if we could not do an in-place update
		if option == nil {
			destructive[alloc.ID] = alloc
			continue
		}

		// Restore the network offers from the existing allocation.
		// We do not allow network resources (reserved/dynamic ports)
		// to be updated. This is guarded in taskUpdated, so we can
		// safely restore those here.
		for task, resources := range option.TaskResources {
			existing := alloc.TaskResources[task]
			resources.Networks = existing.Networks
		}

		// Create a shallow copy
		newAlloc := new(structs.Allocation)
		*newAlloc = *alloc

		// Update the allocation
		newAlloc.EvalID = a.eval.ID
		newAlloc.Job = nil       // Use the Job in the Plan
		newAlloc.Resources = nil // Computed in Plan Apply
		newAlloc.TaskResources = option.TaskResources
		newAlloc.Metrics = a.ctx.Metrics()

		// Add this to the result and the tracking allocSet
		inplace[alloc.ID] = alloc
		a.result.inplaceUpdate = append(a.result.inplaceUpdate, newAlloc)
	}

	return
}
