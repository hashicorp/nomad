package scheduler

import (
	"log"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// allocUpdateType takes an existing allocation and a new job definition and
// returns whether the allocation can ignore the change, requires a destructive
// update, or can be inplace updated. If it can be inplace updated, an updated
// allocation that has the new resources and alloc metrics attached will be
// returned.
type allocUpdateType func(existing *structs.Allocation, newJob *structs.Job, newTG *structs.TaskGroup) (ignore, destructive bool, updated *structs.Allocation)

// allocReconciler is used to determine the set of allocations that require
// placement, inplace updating or stopping given the job specification and
// existing cluster state. The reconciler should only be used for batch and
// service jobs.
type allocReconciler struct {
	// logger is used to log debug information. Logging should be kept at a
	// minimal here
	logger *log.Logger

	// canInplace is used to check if the allocation can be inplace upgraded
	allocUpdateFn allocUpdateType

	// batch marks whether the job is a batch job
	batch bool

	// job is the job being operated on, it may be nil if the job is being
	// stopped via a purge
	job *structs.Job

	// jobID is the ID of the job being operated on. The job may be nil if it is
	// being stopped so we require this seperately.
	jobID string

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

	// desiredTGUpdates captures the desired set of changes to make for each
	// task group.
	desiredTGUpdates map[string]*structs.DesiredUpdates
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
func NewAllocReconciler(logger *log.Logger, allocUpdateFn allocUpdateType, batch bool,
	jobID string, job *structs.Job, deployment *structs.Deployment,
	existingAllocs []*structs.Allocation, taintedNodes map[string]*structs.Node) *allocReconciler {

	a := &allocReconciler{
		logger:         logger,
		allocUpdateFn:  allocUpdateFn,
		batch:          batch,
		jobID:          jobID,
		job:            job,
		deployment:     deployment,
		existingAllocs: existingAllocs,
		taintedNodes:   taintedNodes,
		result: &reconcileResults{
			desiredTGUpdates: make(map[string]*structs.DesiredUpdates),
		},
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
		a.logger.Printf("ALEX: MADE DEPLOYMENT %q", a.deployment.ID)
	}

	if a.deployment != nil {
		a.logger.Printf("ALEX: CURRENT DEPLOYMENT %q", a.deployment.ID)
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
	// Create the desired update object for the group
	desiredChanges := new(structs.DesiredUpdates)
	a.result.desiredTGUpdates[group] = desiredChanges

	// Get the task group. The task group may be nil if the job was updates such
	// that the task group no longer exists
	tg := a.job.LookupTaskGroup(group)

	// Determine what set of alloations are on tainted nodes
	untainted, migrate, lost := as.filterByTainted(a.taintedNodes)

	// If the task group is nil, then the task group has been removed so all we
	// need to do is stop everything
	if tg == nil {
		a.logger.Printf("RECONCILER -- STOPPING ALL")
		a.markStop(untainted, "", allocNotNeeded)
		a.markStop(migrate, "", allocNotNeeded)
		a.markStop(lost, structs.AllocClientStatusLost, allocLost)
		desiredChanges.Stop = uint64(len(untainted) + len(migrate) + len(lost))
		return
	}

	// Get the deployment state for the group
	creatingDeployment := a.result.createDeployment != nil
	var dstate *structs.DeploymentState
	if a.deployment != nil {
		var ok bool
		dstate, ok = a.deployment.TaskGroups[group]

		// We are creating a deployment
		if !ok && creatingDeployment {
			dstate = &structs.DeploymentState{}
			a.deployment.TaskGroups[group] = dstate
		}
	}

	// Track the lost and migrating
	desiredChanges.Migrate += uint64(len(migrate) + len(lost))

	a.logger.Printf("RECONCILER -- untainted (%d); migrate (%d); lost (%d)", len(untainted), len(migrate), len(lost))
	a.logger.Printf("RECONCILER -- untainted %#v", untainted)

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
			desiredChanges.Stop += uint64(len(older))

			a.logger.Printf("RECONCILER -- older canaries %#v", older)
			a.logger.Printf("RECONCILER -- current canaries %#v", current)

			untainted = untainted.difference(older)
			canaries = current
		} else {
			// We don't need any of those canaries since there no longer is a
			// deployment
			a.markStop(canaries, "", allocNotNeeded)
			desiredChanges.Stop += uint64(len(canaries))
			untainted = untainted.difference(canaries)
			canaries = nil
		}
		a.logger.Printf("RECONCILER -- untainted - remove canaries %#v", untainted)
	}

	// Create a structure for choosing names
	nameIndex := newAllocNameIndex(a.jobID, group, tg.Count, untainted)

	// Stop any unneeded allocations and update the untainted set to not
	// included stopped allocations. We ignore canaries since that can push us
	// over the desired count
	existingCanariesPromoted := dstate == nil || dstate.DesiredCanaries == 0 || dstate.Promoted
	stop := a.computeStop(tg, nameIndex, untainted.difference(canaries), canaries, existingCanariesPromoted)
	a.markStop(stop, "", allocNotNeeded)
	desiredChanges.Stop += uint64(len(stop))
	untainted = untainted.difference(stop)

	// Having stopped un-needed allocations, append the canaries to the existing
	// set of untainted because they are promoted. This will cause them to be
	// treated like non-canaries
	if existingCanariesPromoted {
		untainted = untainted.union(canaries)
		nameIndex.Add(canaries)
	}

	// Do inplace upgrades where possible and capture the set of upgrades that
	// need to be done destructively.
	ignore, inplace, destructive := a.computeUpdates(tg, untainted)
	desiredChanges.Ignore += uint64(len(ignore))
	desiredChanges.InPlaceUpdate += uint64(len(inplace))
	desiredChanges.DestructiveUpdate += uint64(len(destructive))

	a.logger.Printf("RECONCILER -- Stopping (%d)", len(stop))
	a.logger.Printf("RECONCILER -- Inplace (%d); Destructive (%d)", len(inplace), len(destructive))

	// Get the update strategy of the group
	strategy := tg.Update

	// The fact that we have destructive updates and have less canaries than is
	// desired means we need to create canaries
	numDestructive := len(destructive)
	requireCanary := numDestructive != 0 && strategy != nil && len(canaries) < strategy.Canary
	if requireCanary && !a.deploymentPaused {
		number := strategy.Canary - len(canaries)
		number = helper.IntMin(numDestructive, number)
		desiredChanges.Canary += uint64(number)
		if creatingDeployment {
			dstate.DesiredCanaries = strategy.Canary
			dstate.DesiredTotal += strategy.Canary
		}

		a.logger.Printf("RECONCILER -- Canary (%d)", number)
		for _, name := range nameIndex.NextCanaries(uint(number), canaries, destructive) {
			a.result.place = append(a.result.place, allocPlaceResult{
				name:      name,
				canary:    true,
				taskGroup: tg,
			})
		}
	}

	// Determine how many we can place
	haveCanaries := dstate != nil && dstate.DesiredCanaries != 0
	limit := a.computeLimit(tg, untainted, destructive, haveCanaries)
	a.logger.Printf("RECONCILER -- LIMIT %v", limit)

	// Place if:
	// * The deployment is not paused
	// * Not placing any canaries
	// * If there are any canaries that they have been promoted
	place := a.computePlacements(tg, nameIndex, untainted)
	if creatingDeployment {
		dstate.DesiredTotal += len(place)
	}

	if !a.deploymentPaused && existingCanariesPromoted {
		// Update the desired changes and if we are creating a deployment update
		// the state.
		desiredChanges.Place += uint64(len(place))

		// Place all new allocations
		a.logger.Printf("RECONCILER -- Placing (%d)", len(place))
		for _, p := range place {
			a.result.place = append(a.result.place, p)
		}

		// Do all destructive updates
		min := helper.IntMin(len(destructive), limit)
		i := 0
		a.logger.Printf("RECONCILER -- Destructive Updating (%d)", min)
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
	a.logger.Printf("RECONCILER -- Migrating (%d)", len(migrate))
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
func (a *allocReconciler) computeLimit(group *structs.TaskGroup, untainted, destructive allocSet, canaries bool) int {
	// If there is no update stategy or deployment for the group we can deploy
	// as many as the group has
	if group.Update == nil || len(destructive) == 0 {
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
func (a *allocReconciler) computePlacements(group *structs.TaskGroup,
	nameIndex *allocNameIndex, untainted allocSet) []allocPlaceResult {

	// Hot path the nothing to do case
	existing := len(untainted)
	if existing >= group.Count {
		return nil
	}

	var place []allocPlaceResult
	for _, name := range nameIndex.Next(uint(group.Count - existing)) {
		place = append(place, allocPlaceResult{
			name:      name,
			taskGroup: group,
		})
	}

	return place
}

// computeStop returns the set of allocations to stop given the group definiton
// and the set of untainted and canary allocations for the group.
func (a *allocReconciler) computeStop(group *structs.TaskGroup, nameIndex *allocNameIndex,
	untainted, canaries allocSet, promoted bool) allocSet {
	// Hot path the nothing to do case
	remove := len(untainted) - group.Count
	if promoted {
		remove += len(canaries)
	}
	if remove <= 0 {
		return nil
	}

	// nameIndex does not include the canaries
	removeNames := nameIndex.Highest(uint(remove))
	stop := make(map[string]*structs.Allocation)
	for id, a := range untainted {
		if _, remove := removeNames[a.Name]; remove {
			stop[id] = a
		}
	}

	return stop
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

	for _, alloc := range untainted {
		ignoreChange, destructiveChange, inplaceAlloc := a.allocUpdateFn(alloc, a.job, group)
		if ignoreChange {
			ignore[alloc.ID] = alloc
		} else if destructiveChange {
			destructive[alloc.ID] = alloc
		} else {
			inplace[alloc.ID] = alloc
			a.result.inplaceUpdate = append(a.result.inplaceUpdate, inplaceAlloc)
		}
	}

	return
}

// allocNameIndex is used to select allocation names for placement or removal
// given an existing set of placed allocations.
type allocNameIndex struct {
	job, taskGroup string
	count          int
	b              structs.Bitmap
}

// newAllocNameIndex returns an allocNameIndex for use in selecting names of
// allocations to create or stop. It takes the job and task group name, desired
// count and any existing allocations as input.
func newAllocNameIndex(job, taskGroup string, count int, in allocSet) *allocNameIndex {
	return &allocNameIndex{
		count:     count,
		b:         bitmapFrom(in, uint(count)),
		job:       job,
		taskGroup: taskGroup,
	}
}

func bitmapFrom(input allocSet, minSize uint) structs.Bitmap {
	var max uint
	for _, a := range input {
		if num := a.Index(); num > max {
			max = num
		}
	}

	if l := uint(len(input)); minSize < l {
		minSize = l
	}
	if max < minSize {
		max = minSize
	}
	if max == 0 {
		max = 8
	}

	// byteAlign the count
	if remainder := max % 8; remainder != 0 {
		max = max + 8 - remainder
	}

	bitmap, err := structs.NewBitmap(max)
	if err != nil {
		panic(err)
	}

	for _, a := range input {
		bitmap.Set(a.Index())
	}

	return bitmap
}

// Add adds the allocations to the name index
func (a *allocNameIndex) Add(set allocSet) {
	for _, alloc := range set {
		a.b.Set(alloc.Index())
	}
}

// RemoveHighest removes and returns the hightest n used names. The returned set
// can be less than n if there aren't n names set in the index
func (a *allocNameIndex) Highest(n uint) map[string]struct{} {
	h := make(map[string]struct{}, n)
	for i := a.b.Size(); i > uint(0) && uint(len(h)) <= n; i-- {
		// Use this to avoid wrapping around b/c of the unsigned int
		idx := i - 1
		if a.b.Check(idx) {
			a.b.Unset(idx)
			h[structs.AllocName(a.job, a.taskGroup, idx)] = struct{}{}
		}
	}

	return h
}

// NextCanaries returns the next n names for use as canaries and sets them as
// used. The existing canaries and destructive updates are also passed in.
func (a *allocNameIndex) NextCanaries(n uint, existing, destructive allocSet) []string {
	next := make([]string, 0, n)

	// First select indexes from the allocations that are undergoing destructive
	// updates. This way we avoid duplicate names as they will get replaced.
	dmap := bitmapFrom(destructive, uint(a.count))
	var remainder uint
	for _, idx := range dmap.IndexesInRange(true, uint(0), uint(a.count)-1) {
		name := structs.AllocName(a.job, a.taskGroup, uint(idx))
		if _, used := existing[name]; !used {
			next = append(next, name)
			a.b.Set(uint(idx))

			// If we have enough, return
			remainder := n - uint(len(next))
			if remainder == 0 {
				return next
			}
		}
	}

	// Get the set of unset names that can be used
	for _, idx := range a.b.IndexesInRange(false, uint(0), uint(a.count)-1) {
		name := structs.AllocName(a.job, a.taskGroup, uint(idx))
		if _, used := existing[name]; !used {
			next = append(next, name)
			a.b.Set(uint(idx))

			// If we have enough, return
			remainder = n - uint(len(next))
			if remainder == 0 {
				return next
			}
		}
	}

	// We have exhausted the prefered and free set, now just pick overlapping
	// indexes
	var i uint
	for i = 0; i < remainder; i++ {
		name := structs.AllocName(a.job, a.taskGroup, i)
		if _, used := existing[name]; !used {
			next = append(next, name)
			a.b.Set(i)

			// If we have enough, return
			remainder = n - uint(len(next))
			if remainder == 0 {
				return next
			}
		}
	}

	return next
}

// Next returns the next n names for use as new placements and sets them as
// used.
func (a *allocNameIndex) Next(n uint) []string {
	next := make([]string, 0, n)

	// Get the set of unset names that can be used
	var remainder uint
	for _, idx := range a.b.IndexesInRange(false, uint(0), uint(a.count)-1) {
		next = append(next, structs.AllocName(a.job, a.taskGroup, uint(idx)))
		a.b.Set(uint(idx))

		// If we have enough, return
		remainder := n - uint(len(next))
		if remainder == 0 {
			return next
		}
	}

	// We have exhausted the free set, now just pick overlapping indexes
	var i uint
	for i = 0; i < remainder; i++ {
		next = append(next, structs.AllocName(a.job, a.taskGroup, i))
		a.b.Set(i)
	}

	return next
}
