package scheduler

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// allocUpdateType takes an existing allocation and a new job definition and
// returns whether the allocation can ignore the change, requires a destructive
// update, or can be inplace updated. If it can be inplace updated, an updated
// allocation that has the new resources and alloc metrics attached will be
// returned.
type allocUpdateType func(existing *structs.Allocation, newJob *structs.Job,
	newTG *structs.TaskGroup) (ignore, destructive bool, updated *structs.Allocation)

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
	// being stopped so we require this separately.
	jobID string

	// oldDeployment is the last deployment for the job
	oldDeployment *structs.Deployment

	// deployment is the current deployment for the job
	deployment *structs.Deployment

	// deploymentPaused marks whether the deployment is paused
	deploymentPaused bool

	// deploymentFailed marks whether the deployment is failed
	deploymentFailed bool

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
	// deployment is the deployment that should be created or updated as a
	// result of scheduling
	deployment *structs.Deployment

	// deploymentUpdates contains a set of deployment updates that should be
	// applied as a result of scheduling
	deploymentUpdates []*structs.DeploymentStatusUpdate

	// place is the set of allocations to place by the scheduler
	place []allocPlaceResult

	// destructiveUpdate is the set of allocations to apply a destructive update to
	destructiveUpdate []allocDestructiveResult

	// inplaceUpdate is the set of allocations to apply an inplace update to
	inplaceUpdate []*structs.Allocation

	// stop is the set of allocations to stop
	stop []allocStopResult

	// desiredTGUpdates captures the desired set of changes to make for each
	// task group.
	desiredTGUpdates map[string]*structs.DesiredUpdates

	// followupEvalWait is set if there should be a followup eval run after the
	// given duration
	followupEvalWait time.Duration
}

func (r *reconcileResults) GoString() string {
	base := fmt.Sprintf("Total changes: (place %d) (destructive %d) (inplace %d) (stop %d)",
		len(r.place), len(r.destructiveUpdate), len(r.inplaceUpdate), len(r.stop))

	if r.deployment != nil {
		base += fmt.Sprintf("\nCreated Deployment: %q", r.deployment.ID)
	}
	for _, u := range r.deploymentUpdates {
		base += fmt.Sprintf("\nDeployment Update for ID %q: Status %q; Description %q",
			u.DeploymentID, u.Status, u.StatusDescription)
	}
	if r.followupEvalWait != 0 {
		base += fmt.Sprintf("\nFollowup Eval in %v", r.followupEvalWait)
	}
	for tg, u := range r.desiredTGUpdates {
		base += fmt.Sprintf("\nDesired Changes for %q: %#v", tg, u)
	}
	return base
}

// Changes returns the number of total changes
func (r *reconcileResults) Changes() int {
	return len(r.place) + len(r.inplaceUpdate) + len(r.stop)
}

// NewAllocReconciler creates a new reconciler that should be used to determine
// the changes required to bring the cluster state inline with the declared jobspec
func NewAllocReconciler(logger *log.Logger, allocUpdateFn allocUpdateType, batch bool,
	jobID string, job *structs.Job, deployment *structs.Deployment,
	existingAllocs []*structs.Allocation, taintedNodes map[string]*structs.Node) *allocReconciler {

	return &allocReconciler{
		logger:         logger,
		allocUpdateFn:  allocUpdateFn,
		batch:          batch,
		jobID:          jobID,
		job:            job,
		deployment:     deployment.Copy(),
		existingAllocs: existingAllocs,
		taintedNodes:   taintedNodes,
		result: &reconcileResults{
			desiredTGUpdates: make(map[string]*structs.DesiredUpdates),
		},
	}
}

// Compute reconciles the existing cluster state and returns the set of changes
// required to converge the job spec and state
func (a *allocReconciler) Compute() *reconcileResults {
	// Create the allocation matrix
	m := newAllocMatrix(a.job, a.existingAllocs)

	// Handle stopping unneeded deployments
	a.cancelDeployments()

	// If we are just stopping a job we do not need to do anything more than
	// stopping all running allocs
	if a.job.Stopped() {
		a.handleStop(m)
		return a.result
	}

	// Detect if the deployment is paused
	if a.deployment != nil {
		a.deploymentPaused = a.deployment.Status == structs.DeploymentStatusPaused
		a.deploymentFailed = a.deployment.Status == structs.DeploymentStatusFailed
	}

	// Reconcile each group
	complete := true
	for group, as := range m {
		groupComplete := a.computeGroup(group, as)
		complete = complete && groupComplete
	}

	// Mark the deployment as complete if possible
	if a.deployment != nil && complete {
		a.result.deploymentUpdates = append(a.result.deploymentUpdates, &structs.DeploymentStatusUpdate{
			DeploymentID:      a.deployment.ID,
			Status:            structs.DeploymentStatusSuccessful,
			StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
		})
	}

	// Set the description of a created deployment
	if d := a.result.deployment; d != nil {
		if d.RequiresPromotion() {
			d.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
		}
	}

	return a.result
}

// cancelDeployments cancels any deployment that is not needed
func (a *allocReconciler) cancelDeployments() {
	// If the job is stopped and there is a non-terminal deployment, cancel it
	if a.job.Stopped() {
		if a.deployment != nil && a.deployment.Active() {
			a.result.deploymentUpdates = append(a.result.deploymentUpdates, &structs.DeploymentStatusUpdate{
				DeploymentID:      a.deployment.ID,
				Status:            structs.DeploymentStatusCancelled,
				StatusDescription: structs.DeploymentStatusDescriptionStoppedJob,
			})
		}

		// Nothing else to do
		a.oldDeployment = a.deployment
		a.deployment = nil
		return
	}

	d := a.deployment
	if d == nil {
		return
	}

	// Check if the deployment is active and referencing an older job and cancel it
	if d.JobCreateIndex != a.job.CreateIndex || d.JobVersion != a.job.Version {
		if d.Active() {
			a.result.deploymentUpdates = append(a.result.deploymentUpdates, &structs.DeploymentStatusUpdate{
				DeploymentID:      a.deployment.ID,
				Status:            structs.DeploymentStatusCancelled,
				StatusDescription: structs.DeploymentStatusDescriptionNewerJob,
			})
		}

		a.oldDeployment = d
		a.deployment = nil
	}

	// Clear it as the current deployment if it is successful
	if d.Status == structs.DeploymentStatusSuccessful {
		a.oldDeployment = d
		a.deployment = nil
	}
}

// handleStop marks all allocations to be stopped, handling the lost case
func (a *allocReconciler) handleStop(m allocMatrix) {
	for group, as := range m {
		untainted, migrate, lost := as.filterByTainted(a.taintedNodes)
		a.markStop(untainted, "", allocNotNeeded)
		a.markStop(migrate, "", allocNotNeeded)
		a.markStop(lost, structs.AllocClientStatusLost, allocLost)
		desiredChanges := new(structs.DesiredUpdates)
		desiredChanges.Stop = uint64(len(as))
		a.result.desiredTGUpdates[group] = desiredChanges
	}
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

// computeGroup reconciles state for a particular task group. It returns whether
// the deployment it is for is complete with regards to the task group.
func (a *allocReconciler) computeGroup(group string, all allocSet) bool {
	// Create the desired update object for the group
	desiredChanges := new(structs.DesiredUpdates)
	a.result.desiredTGUpdates[group] = desiredChanges

	// Get the task group. The task group may be nil if the job was updates such
	// that the task group no longer exists
	tg := a.job.LookupTaskGroup(group)

	// If the task group is nil, then the task group has been removed so all we
	// need to do is stop everything
	if tg == nil {
		untainted, migrate, lost := all.filterByTainted(a.taintedNodes)
		a.markStop(untainted, "", allocNotNeeded)
		a.markStop(migrate, "", allocNotNeeded)
		a.markStop(lost, structs.AllocClientStatusLost, allocLost)
		desiredChanges.Stop = uint64(len(untainted) + len(migrate) + len(lost))
		return true
	}

	// Get the deployment state for the group
	var dstate *structs.DeploymentState
	existingDeployment := false
	if a.deployment != nil {
		dstate, existingDeployment = a.deployment.TaskGroups[group]
	}
	if !existingDeployment {
		autorevert := false
		if tg.Update != nil && tg.Update.AutoRevert {
			autorevert = true
		}
		dstate = &structs.DeploymentState{
			AutoRevert: autorevert,
		}
	}

	canaries, all := a.handleGroupCanaries(all, desiredChanges)

	// Determine what set of allocations are on tainted nodes
	untainted, migrate, lost := all.filterByTainted(a.taintedNodes)

	// Create a structure for choosing names. Seed with the taken names which is
	// the union of untainted and migrating nodes (includes canaries)
	nameIndex := newAllocNameIndex(a.jobID, group, tg.Count, untainted.union(migrate))

	// Stop any unneeded allocations and update the untainted set to not
	// included stopped allocations.
	canaryState := dstate != nil && dstate.DesiredCanaries != 0 && !dstate.Promoted
	stop := a.computeStop(tg, nameIndex, untainted, migrate, lost, canaries, canaryState)
	desiredChanges.Stop += uint64(len(stop))
	untainted = untainted.difference(stop)

	// Having stopped un-needed allocations, append the canaries to the existing
	// set of untainted because they are promoted. This will cause them to be
	// treated like non-canaries
	if !canaryState {
		untainted = untainted.union(canaries)
		nameIndex.Set(canaries)
	}

	// Do inplace upgrades where possible and capture the set of upgrades that
	// need to be done destructively.
	ignore, inplace, destructive := a.computeUpdates(tg, untainted)
	desiredChanges.Ignore += uint64(len(ignore))
	desiredChanges.InPlaceUpdate += uint64(len(inplace))
	if !existingDeployment {
		dstate.DesiredTotal += len(destructive) + len(inplace)
	}

	// The fact that we have destructive updates and have less canaries than is
	// desired means we need to create canaries
	numDestructive := len(destructive)
	strategy := tg.Update
	canariesPromoted := dstate != nil && dstate.Promoted
	requireCanary := numDestructive != 0 && strategy != nil && len(canaries) < strategy.Canary && !canariesPromoted
	if requireCanary && !a.deploymentPaused && !a.deploymentFailed {
		number := strategy.Canary - len(canaries)
		number = helper.IntMin(numDestructive, number)
		desiredChanges.Canary += uint64(number)
		if !existingDeployment {
			dstate.DesiredCanaries = strategy.Canary
		}

		for _, name := range nameIndex.NextCanaries(uint(number), canaries, destructive) {
			a.result.place = append(a.result.place, allocPlaceResult{
				name:      name,
				canary:    true,
				taskGroup: tg,
			})
		}
	}

	// Determine how many we can place
	canaryState = dstate != nil && dstate.DesiredCanaries != 0 && !dstate.Promoted
	limit := a.computeLimit(tg, untainted, destructive, migrate, canaryState)

	// Place if:
	// * The deployment is not paused or failed
	// * Not placing any canaries
	// * If there are any canaries that they have been promoted
	place := a.computePlacements(tg, nameIndex, untainted, migrate)
	if !existingDeployment {
		dstate.DesiredTotal += len(place)
	}

	// deploymentPlaceReady tracks whether the deployment is in a state where
	// placements can be made without any other consideration.
	deploymentPlaceReady := !a.deploymentPaused && !a.deploymentFailed && !canaryState

	if deploymentPlaceReady {
		desiredChanges.Place += uint64(len(place))
		for _, p := range place {
			a.result.place = append(a.result.place, p)
		}

		min := helper.IntMin(len(place), limit)
		limit -= min
	} else if !deploymentPlaceReady && len(lost) != 0 {
		// We are in a situation where we shouldn't be placing more than we need
		// to but we have lost allocations. It is a very weird user experience
		// if you have a node go down and Nomad doesn't replace the allocations
		// because the deployment is paused/failed so we only place to recover
		// the lost allocations.
		allowed := helper.IntMin(len(lost), len(place))
		desiredChanges.Place += uint64(allowed)
		for _, p := range place[:allowed] {
			a.result.place = append(a.result.place, p)
		}
	}

	if deploymentPlaceReady {
		// Do all destructive updates
		min := helper.IntMin(len(destructive), limit)
		limit -= min
		desiredChanges.DestructiveUpdate += uint64(min)
		desiredChanges.Ignore += uint64(len(destructive) - min)
		for _, alloc := range destructive.nameOrder()[:min] {
			a.result.destructiveUpdate = append(a.result.destructiveUpdate, allocDestructiveResult{
				placeName:             alloc.Name,
				placeTaskGroup:        tg,
				stopAlloc:             alloc,
				stopStatusDescription: allocUpdating,
			})
		}
	} else {
		desiredChanges.Ignore += uint64(len(destructive))
	}

	// Calculate the allowed number of changes and set the desired changes
	// accordingly.
	min := helper.IntMin(len(migrate), limit)
	if !a.deploymentFailed && !a.deploymentPaused {
		desiredChanges.Migrate += uint64(min)
		desiredChanges.Ignore += uint64(len(migrate) - min)
	} else {
		desiredChanges.Stop += uint64(len(migrate))
	}

	followup := false
	migrated := 0
	for _, alloc := range migrate.nameOrder() {
		// If the deployment is failed or paused, don't replace it, just mark as stop.
		if a.deploymentFailed || a.deploymentPaused {
			a.result.stop = append(a.result.stop, allocStopResult{
				alloc:             alloc,
				statusDescription: allocNodeTainted,
			})
			continue
		}

		if migrated >= limit {
			followup = true
			break
		}

		migrated++
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

	// We need to create a followup evaluation.
	if followup && strategy != nil && a.result.followupEvalWait < strategy.Stagger {
		a.result.followupEvalWait = strategy.Stagger
	}

	// Create a new deployment if necessary
	if !existingDeployment && strategy != nil && dstate.DesiredTotal != 0 {
		// A previous group may have made the deployment already
		if a.deployment == nil {
			a.deployment = structs.NewDeployment(a.job)
			a.result.deployment = a.deployment
		}

		// Attach the groups deployment state to the deployment
		a.deployment.TaskGroups[group] = dstate
	}

	// deploymentComplete is whether the deployment is complete which largely
	// means that no placements were made or desired to be made
	deploymentComplete := len(destructive)+len(inplace)+len(place)+len(migrate) == 0 && !requireCanary

	// Final check to see if the deployment is complete is to ensure everything
	// is healthy
	if deploymentComplete && a.deployment != nil {
		partOf, _ := untainted.filterByDeployment(a.deployment.ID)
		for _, alloc := range partOf {
			if !alloc.DeploymentStatus.IsHealthy() {
				deploymentComplete = false
				break
			}
		}
	}

	return deploymentComplete
}

// handleGroupCanaries handles the canaries for the group by stopping the
// unneeded ones and returning the current set of canaries and the updated total
// set of allocs for the group
func (a *allocReconciler) handleGroupCanaries(all allocSet, desiredChanges *structs.DesiredUpdates) (canaries, newAll allocSet) {
	// Stop any canary from an older deployment or from a failed one
	var stop []string

	// Cancel any non-promoted canaries from the older deployment
	if a.oldDeployment != nil {
		for _, s := range a.oldDeployment.TaskGroups {
			if !s.Promoted {
				stop = append(stop, s.PlacedCanaries...)
			}
		}
	}

	// Cancel any non-promoted canaries from a failed deployment
	if a.deployment != nil && a.deployment.Status == structs.DeploymentStatusFailed {
		for _, s := range a.deployment.TaskGroups {
			if !s.Promoted {
				stop = append(stop, s.PlacedCanaries...)
			}
		}
	}

	// stopSet is the allocSet that contains the canaries we desire to stop from
	// above.
	stopSet := all.fromKeys(stop)
	a.markStop(stopSet, "", allocNotNeeded)
	desiredChanges.Stop += uint64(len(stopSet))
	all = all.difference(stopSet)

	// Capture our current set of canaries and handle any migrations that are
	// needed by just stopping them.
	if a.deployment != nil {
		var canaryIDs []string
		for _, s := range a.deployment.TaskGroups {
			canaryIDs = append(canaryIDs, s.PlacedCanaries...)
		}

		canaries = all.fromKeys(canaryIDs)
		untainted, migrate, lost := canaries.filterByTainted(a.taintedNodes)
		a.markStop(migrate, "", allocMigrating)
		a.markStop(lost, structs.AllocClientStatusLost, allocLost)

		canaries = untainted
		all = all.difference(migrate, lost)
	}

	return canaries, all
}

// computeLimit returns the placement limit for a particular group. The inputs
// are the group definition, the untainted, destructive, and migrate allocation
// set and whether we are in a canary state.
func (a *allocReconciler) computeLimit(group *structs.TaskGroup, untainted, destructive, migrate allocSet, canaryState bool) int {
	// If there is no update stategy or deployment for the group we can deploy
	// as many as the group has
	if group.Update == nil || len(destructive)+len(migrate) == 0 {
		return group.Count
	} else if a.deploymentPaused || a.deploymentFailed {
		// If the deployment is paused or failed, do not create anything else
		return 0
	}

	// If we have canaries and they have not been promoted the limit is 0
	if canaryState {
		return 0
	}

	// If we have been promoted or there are no canaries, the limit is the
	// configured MaxParallel minus any outstanding non-healthy alloc for the
	// deployment
	limit := group.Update.MaxParallel
	if a.deployment != nil {
		partOf, _ := untainted.filterByDeployment(a.deployment.ID)
		for _, alloc := range partOf {
			// An unhealthy allocation means nothing else should be happen.
			if alloc.DeploymentStatus.IsUnhealthy() {
				return 0
			}

			if !alloc.DeploymentStatus.IsHealthy() {
				limit--
			}
		}
	}

	// The limit can be less than zero in the case that the job was changed such
	// that it required destructive changes and the count was scaled up.
	if limit < 0 {
		return 0
	}

	return limit
}

// computePlacement returns the set of allocations to place given the group
// definition, the set of untainted and migrating allocations for the group.
func (a *allocReconciler) computePlacements(group *structs.TaskGroup,
	nameIndex *allocNameIndex, untainted, migrate allocSet) []allocPlaceResult {

	// Hot path the nothing to do case
	existing := len(untainted) + len(migrate)
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

// computeStop returns the set of allocations that are marked for stopping given
// the group definition, the set of allocations in various states and whether we
// are canarying.
func (a *allocReconciler) computeStop(group *structs.TaskGroup, nameIndex *allocNameIndex,
	untainted, migrate, lost, canaries allocSet, canaryState bool) allocSet {

	// Mark all lost allocations for stop. Previous allocation doesn't matter
	// here since it is on a lost node
	var stop allocSet
	stop = stop.union(lost)
	a.markStop(lost, structs.AllocClientStatusLost, allocLost)

	// If we are still deploying or creating canaries, don't stop them
	if canaryState {
		untainted = untainted.difference(canaries)
	}

	// Hot path the nothing to do case
	remove := len(untainted) + len(migrate) - group.Count
	if remove <= 0 {
		return stop
	}

	// Prefer stopping any alloc that has the same name as the canaries if we
	// are promoted
	if !canaryState && len(canaries) != 0 {
		canaryNames := canaries.nameSet()
		for id, alloc := range untainted.difference(canaries) {
			if _, match := canaryNames[alloc.Name]; match {
				stop[id] = alloc
				a.result.stop = append(a.result.stop, allocStopResult{
					alloc:             alloc,
					statusDescription: allocNotNeeded,
				})
				delete(untainted, id)

				remove--
				if remove == 0 {
					return stop
				}
			}
		}
	}

	// Prefer selecting from the migrating set before stopping existing allocs
	if len(migrate) != 0 {
		mNames := newAllocNameIndex(a.jobID, group.Name, group.Count, migrate)
		removeNames := mNames.Highest(uint(remove))
		for id, alloc := range migrate {
			if _, match := removeNames[alloc.Name]; !match {
				continue
			}
			a.result.stop = append(a.result.stop, allocStopResult{
				alloc:             alloc,
				statusDescription: allocNotNeeded,
			})
			delete(migrate, id)
			stop[id] = alloc
			nameIndex.UnsetIndex(alloc.Index())

			remove--
			if remove == 0 {
				return stop
			}
		}
	}

	// Select the allocs with the highest count to remove
	removeNames := nameIndex.Highest(uint(remove))
	for id, alloc := range untainted {
		if _, remove := removeNames[alloc.Name]; remove {
			stop[id] = alloc
			a.result.stop = append(a.result.stop, allocStopResult{
				alloc:             alloc,
				statusDescription: allocNotNeeded,
			})
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
			// Attach the deployment ID and and clear the health if the
			// deployment has changed
			inplace[alloc.ID] = alloc
			a.result.inplaceUpdate = append(a.result.inplaceUpdate, inplaceAlloc)
		}
	}

	return
}
