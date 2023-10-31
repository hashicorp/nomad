// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

// The reconciler is the first stage in the scheduler for service and batch
// jobs. It compares the existing state to the desired state to determine the
// set of changes needed. System jobs and sysbatch jobs do not use the
// reconciler.

import (
	"fmt"
	"sort"
	"time"

	"github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler/generic/alloc"
)

const (
	// batchedFailedAllocWindowSize is the window size used
	// to batch up failed allocations before creating an eval
	batchedFailedAllocWindowSize = 5 * time.Second
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
	logger log.Logger

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

	// evalID and evalPriority is the ID and Priority of the evaluation that
	// triggered the reconciler.
	evalID       string
	evalPriority int

	// supportsDisconnectedClients indicates whether all servers meet the required
	// minimum version to allow application of max_client_disconnect configuration.
	supportsDisconnectedClients bool

	// now is the time used when determining rescheduling eligibility
	// defaults to time.Now, and overridden in unit tests
	now time.Time

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

	// attributeUpdates are updates to the allocation that are not from a
	// jobspec change.
	attributeUpdates map[string]*structs.Allocation

	// disconnectUpdates is the set of allocations are on disconnected nodes, but
	// have not yet had their ClientStatus set to AllocClientStatusUnknown.
	disconnectUpdates map[string]*structs.Allocation

	// reconnectUpdates is the set of allocations that have ClientStatus set to
	// AllocClientStatusUnknown, but the associated Node has reconnected.
	reconnectUpdates map[string]*structs.Allocation

	// desiredTGUpdates captures the desired set of changes to make for each
	// task group.
	desiredTGUpdates map[string]*structs.DesiredUpdates

	// desiredFollowupEvals is the map of follow up evaluations to create per task group
	// This is used to create a delayed evaluation for rescheduling failed allocations.
	desiredFollowupEvals map[string][]*structs.Evaluation

	// taskGroupAllocNameIndexes is a tracking of the allocation name index,
	// keyed by the task group name. This is stored within the results, so the
	// generic scheduler can use this to perform duplicate alloc index checks
	// before submitting the plan. This is always non-nil and is handled within
	// a single routine, so does not require a mutex.
	taskGroupAllocNameIndexes map[string]*alloc.NameIndex
}

func (r *reconcileResults) GoString() string {
	base := fmt.Sprintf("Total changes: (place %d) (destructive %d) (inplace %d) (stop %d) (disconnect %d) (reconnect %d)",
		len(r.place), len(r.destructiveUpdate), len(r.inplaceUpdate), len(r.stop), len(r.disconnectUpdates), len(r.reconnectUpdates))

	if r.deployment != nil {
		base += fmt.Sprintf("\nCreated Deployment: %q", r.deployment.ID)
	}
	for _, u := range r.deploymentUpdates {
		base += fmt.Sprintf("\nDeployment Update for ID %q: Status %q; Description %q",
			u.DeploymentID, u.Status, u.StatusDescription)
	}
	for tg, u := range r.desiredTGUpdates {
		base += fmt.Sprintf("\nDesired Changes for %q: %#v", tg, u)
	}
	return base
}

// NewAllocReconciler creates a new reconciler that should be used to determine
// the changes required to bring the cluster state inline with the declared jobspec
func NewAllocReconciler(logger log.Logger, allocUpdateFn allocUpdateType, batch bool,
	jobID string, job *structs.Job, deployment *structs.Deployment,
	existingAllocs []*structs.Allocation, taintedNodes map[string]*structs.Node, evalID string,
	evalPriority int, supportsDisconnectedClients bool) *allocReconciler {
	return &allocReconciler{
		logger:                      logger.Named("reconciler"),
		allocUpdateFn:               allocUpdateFn,
		batch:                       batch,
		jobID:                       jobID,
		job:                         job,
		deployment:                  deployment.Copy(),
		existingAllocs:              existingAllocs,
		taintedNodes:                taintedNodes,
		evalID:                      evalID,
		evalPriority:                evalPriority,
		supportsDisconnectedClients: supportsDisconnectedClients,
		now:                         time.Now(),
		result: &reconcileResults{
			attributeUpdates:          make(map[string]*structs.Allocation),
			disconnectUpdates:         make(map[string]*structs.Allocation),
			reconnectUpdates:          make(map[string]*structs.Allocation),
			desiredTGUpdates:          make(map[string]*structs.DesiredUpdates),
			desiredFollowupEvals:      make(map[string][]*structs.Evaluation),
			taskGroupAllocNameIndexes: make(map[string]*alloc.NameIndex),
		},
	}
}

// Compute reconciles the existing cluster state and returns the set of changes
// required to converge the job spec and state
func (a *allocReconciler) Compute() *reconcileResults {
	// Create the allocation matrix
	m := alloc.NewMatrix(a.job, a.existingAllocs)

	a.cancelUnneededDeployments()

	// If we are just stopping a job we do not need to do anything more than
	// stopping all running allocs
	if a.job.Stopped() {
		a.handleStop(m)
		return a.result
	}

	a.computeDeploymentPaused()
	deploymentComplete := a.computeDeploymentComplete(m)
	a.computeDeploymentUpdates(deploymentComplete)

	return a.result
}

func (a *allocReconciler) computeDeploymentComplete(m alloc.Matrix) bool {
	complete := true
	for group, as := range m {
		groupComplete := a.computeGroup(group, as)
		complete = complete && groupComplete
	}

	return complete
}

func (a *allocReconciler) computeDeploymentUpdates(deploymentComplete bool) {
	if a.deployment != nil {
		// Mark the deployment as complete if possible
		if deploymentComplete {
			if a.job.IsMultiregion() {
				// the unblocking/successful states come after blocked, so we
				// need to make sure we don't revert those states
				if a.deployment.Status != structs.DeploymentStatusUnblocking &&
					a.deployment.Status != structs.DeploymentStatusSuccessful {
					a.result.deploymentUpdates = append(a.result.deploymentUpdates, &structs.DeploymentStatusUpdate{
						DeploymentID:      a.deployment.ID,
						Status:            structs.DeploymentStatusBlocked,
						StatusDescription: structs.DeploymentStatusDescriptionBlocked,
					})
				}
			} else {
				a.result.deploymentUpdates = append(a.result.deploymentUpdates, &structs.DeploymentStatusUpdate{
					DeploymentID:      a.deployment.ID,
					Status:            structs.DeploymentStatusSuccessful,
					StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
				})
			}
		}

		// Mark the deployment as pending since its state is now computed.
		if a.deployment.Status == structs.DeploymentStatusInitializing {
			a.result.deploymentUpdates = append(a.result.deploymentUpdates, &structs.DeploymentStatusUpdate{
				DeploymentID:      a.deployment.ID,
				Status:            structs.DeploymentStatusPending,
				StatusDescription: structs.DeploymentStatusDescriptionPendingForPeer,
			})
		}
	}

	// Set the description of a created deployment
	if d := a.result.deployment; d != nil {
		if d.RequiresPromotion() {
			if d.HasAutoPromote() {
				d.StatusDescription = structs.DeploymentStatusDescriptionRunningAutoPromotion
			} else {
				d.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
			}
		}
	}
}

// computeDeploymentPaused is responsible for setting flags on the
// allocReconciler that indicate the state of the deployment if one
// is required. The flags that are managed are:
//  1. deploymentFailed: Did the current deployment fail just as named.
//  2. deploymentPaused: Set to true when the current deployment is paused,
//     which is usually a manual user operation, or if the deployment is
//     pending or initializing, which are the initial states for multi-region
//     job deployments. This flag tells Compute that we should not make
//     placements on the deployment.
func (a *allocReconciler) computeDeploymentPaused() {
	if a.deployment != nil {
		a.deploymentPaused = a.deployment.Status == structs.DeploymentStatusPaused ||
			a.deployment.Status == structs.DeploymentStatusPending ||
			a.deployment.Status == structs.DeploymentStatusInitializing
		a.deploymentFailed = a.deployment.Status == structs.DeploymentStatusFailed
	}
}

// cancelUnneededDeployments cancels any deployment that is not needed. If the
// current deployment is not needed the deployment field is set to nil. A deployment
// update will be staged for jobs that should stop or have the wrong version.
// Unneeded deployments include:
// 1. Jobs that are marked for stop, but there is a non-terminal deployment.
// 2. Deployments that are active, but referencing a different job version.
// 3. Deployments that are already successful.
func (a *allocReconciler) cancelUnneededDeployments() {
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
	// Clear it as the current deployment if it is successful
	if d.Status == structs.DeploymentStatusSuccessful {
		a.oldDeployment = d
		a.deployment = nil
	}
}

// handleStop marks all allocations to be stopped, handling the lost case
func (a *allocReconciler) handleStop(m alloc.Matrix) {
	for group, as := range m {
		as = alloc.FilterByTerminal(as)
		desiredChanges := new(structs.DesiredUpdates)
		desiredChanges.Stop = a.filterAndStopAll(as)
		a.result.desiredTGUpdates[group] = desiredChanges
	}
}

// filterAndStopAll stops all allocations in an allocSet. This is useful in when
// stopping an entire job or task group.
func (a *allocReconciler) filterAndStopAll(set alloc.Set) uint64 {
	untainted, migrate, lost, disconnecting, reconnecting, ignore := set.FilterByTainted(a.taintedNodes, a.supportsDisconnectedClients, a.now)
	a.markStop(untainted, "", allocNotNeeded)
	a.markStop(migrate, "", allocNotNeeded)
	a.markStop(lost, structs.AllocClientStatusLost, allocLost)
	a.markStop(disconnecting, "", allocNotNeeded)
	a.markStop(reconnecting, "", allocNotNeeded)
	a.markStop(ignore.FilterByClientStatus(structs.AllocClientStatusUnknown), "", allocNotNeeded)
	return uint64(len(set))
}

// markStop is a helper for marking a set of allocation for stop with a
// particular client status and description.
func (a *allocReconciler) markStop(allocs alloc.Set, clientStatus, statusDescription string) {
	for _, allocEntry := range allocs {
		a.result.stop = append(a.result.stop, allocStopResult{
			alloc:             allocEntry,
			clientStatus:      clientStatus,
			statusDescription: statusDescription,
		})
	}
}

// markDelayed does markStop, but optionally includes a FollowupEvalID so that we can update
// the stopped alloc with its delayed rescheduling evalID
func (a *allocReconciler) markDelayed(allocs alloc.Set, clientStatus, statusDescription string, followupEvals map[string]string) {
	for _, allocEntry := range allocs {
		a.result.stop = append(a.result.stop, allocStopResult{
			alloc:             allocEntry,
			clientStatus:      clientStatus,
			statusDescription: statusDescription,
			followupEvalID:    followupEvals[allocEntry.ID],
		})
	}
}

// computeGroup reconciles state for a particular task group. It returns whether
// the deployment it is for is complete with regards to the task group.
func (a *allocReconciler) computeGroup(groupName string, all alloc.Set) bool {

	// Create the desired update object for the group
	desiredChanges := new(structs.DesiredUpdates)
	a.result.desiredTGUpdates[groupName] = desiredChanges

	// Get the task group. The task group may be nil if the job was updates such
	// that the task group no longer exists
	tg := a.job.LookupTaskGroup(groupName)

	// If the task group is nil, then the task group has been removed so all we
	// need to do is stop everything
	if tg == nil {
		desiredChanges.Stop = a.filterAndStopAll(all)
		return true
	}

	dstate, existingDeployment := a.initializeDeploymentState(groupName, tg)

	// Filter allocations that do not need to be considered because they are
	// from an older job version and are terminal.
	all, ignore := a.filterOldTerminalAllocs(all)
	desiredChanges.Ignore += uint64(len(ignore))

	canaries, all := a.cancelUnneededCanaries(all, desiredChanges)

	// Determine what set of allocations are on tainted nodes
	untainted, migrate, lost, disconnecting, reconnecting, ignore := all.FilterByTainted(a.taintedNodes, a.supportsDisconnectedClients, a.now)
	desiredChanges.Ignore += uint64(len(ignore))

	// Determine what set of terminal allocations need to be rescheduled
	untainted, rescheduleNow, rescheduleLater := untainted.FilterByRescheduleable(a.batch, false, a.now, a.evalID, a.deployment)

	// If there are allocations reconnecting we need to reconcile them and
	// their replacements first because there is specific logic when deciding
	// which ones to keep that can only be applied when the client reconnects.
	if len(reconnecting) > 0 {
		// Pass all allocations because the replacements we need to find may be
		// in any state, including themselves being reconnected.
		reconnect, stop := a.reconcileReconnecting(reconnecting, all)

		// Stop the reconciled allocations and remove them from the other sets
		// since they have been already handled.
		desiredChanges.Stop += uint64(len(stop))

		untainted = untainted.Difference(stop)
		migrate = migrate.Difference(stop)
		lost = lost.Difference(stop)
		disconnecting = disconnecting.Difference(stop)
		reconnecting = reconnecting.Difference(stop)
		ignore = ignore.Difference(stop)

		// Validate and add reconnecting allocations to the plan so they are
		// logged.
		a.computeReconnecting(reconnect)

		// The rest of the reconnecting allocations is now untainted and will
		// be further reconciled below.
		untainted = untainted.Union(reconnect)
	}

	// Determine what set of disconnecting allocations need to be rescheduled now,
	// which ones later and which ones can't be rescheduled at all.
	timeoutLaterEvals := map[string]string{}
	if len(disconnecting) > 0 {
		untaintedDisconnecting, rescheduleDisconnecting, laterDisconnecting := disconnecting.FilterByRescheduleable(a.batch, true, a.now, a.evalID, a.deployment)

		rescheduleNow = rescheduleNow.Union(rescheduleDisconnecting)
		untainted = untainted.Union(untaintedDisconnecting)
		rescheduleLater = append(rescheduleLater, laterDisconnecting...)

		// Find delays for any disconnecting allocs that have max_client_disconnect,
		// create followup evals, and update the ClientStatus to unknown.
		timeoutLaterEvals = a.createTimeoutLaterEvals(disconnecting, tg.Name)
	}

	// Find delays for any lost allocs that have stop_after_client_disconnect
	lostLaterEvals := map[string]string{}
	lostLater := []*alloc.DelayedRescheduleInfo{}
	if len(lost) > 0 {
		lostLater = lost.DelayByStopAfterClientDisconnect()
		lostLaterEvals = a.createLostLaterEvals(lostLater, tg.Name)
	}

	// Merge disconnecting with the stop_after_client_disconnect set into the
	// lostLaterEvals so that computeStop can add them to the stop set.
	lostLaterEvals = helper.MergeMapStringString(lostLaterEvals, timeoutLaterEvals)

	// Create batched follow-up evaluations for allocations that are
	// reschedulable later and mark the allocations for in place updating
	a.createRescheduleLaterEvals(rescheduleLater, all, tg.Name)

	// Create a structure for choosing names. Seed with the taken names
	// which is the union of untainted, rescheduled, allocs on migrating
	// nodes, and allocs on down nodes (includes canaries)
	nameIndex := alloc.NewNameIndex(a.jobID, groupName, tg.Count, untainted.Union(migrate, rescheduleNow, lost))
	a.result.taskGroupAllocNameIndexes[groupName] = nameIndex

	// Stop any unneeded allocations and update the untainted set to not
	// include stopped allocations.
	isCanarying := dstate != nil && dstate.DesiredCanaries != 0 && !dstate.Promoted
	stop := a.computeStop(tg, nameIndex, untainted, migrate, lost, canaries, isCanarying, lostLaterEvals)

	desiredChanges.Stop += uint64(len(stop))
	untainted = untainted.Difference(stop)

	// Do inplace upgrades where possible and capture the set of upgrades that
	// need to be done destructively.
	ignoreUpdates, inplace, destructive := a.computeUpdates(tg, untainted)

	desiredChanges.Ignore += uint64(len(ignoreUpdates))
	desiredChanges.InPlaceUpdate += uint64(len(inplace))
	if !existingDeployment {
		dstate.DesiredTotal += len(destructive) + len(inplace)
	}

	// Remove the canaries now that we have handled rescheduling so that we do
	// not consider them when making placement decisions.
	if isCanarying {
		untainted = untainted.Difference(canaries)
	}
	requiresCanaries := a.requiresCanaries(tg, dstate, destructive, canaries)
	if requiresCanaries {
		a.computeCanaries(tg, dstate, destructive, canaries, desiredChanges, nameIndex)
	}

	// Determine how many non-canary allocs we can place
	isCanarying = dstate != nil && dstate.DesiredCanaries != 0 && !dstate.Promoted
	underProvisionedBy := a.computeUnderProvisionedBy(tg, untainted, destructive, migrate, isCanarying)

	// Place if:
	// * The deployment is not paused or failed
	// * Not placing any canaries
	// * If there are any canaries that they have been promoted
	// * There is no delayed stop_after_client_disconnect alloc, which delays scheduling for the whole group
	// * An alloc was lost
	var place []allocPlaceResult
	if len(lostLater) == 0 {
		place = a.computePlacements(tg, nameIndex, untainted, migrate, rescheduleNow, lost, isCanarying)
		if !existingDeployment {
			dstate.DesiredTotal += len(place)
		}
	}

	// deploymentPlaceReady tracks whether the deployment is in a state where
	// placements can be made without any other consideration.
	deploymentPlaceReady := !a.deploymentPaused && !a.deploymentFailed && !isCanarying

	underProvisionedBy = a.computeReplacements(deploymentPlaceReady, desiredChanges, place, rescheduleNow, lost, underProvisionedBy)

	if deploymentPlaceReady {
		a.computeDestructiveUpdates(destructive, underProvisionedBy, desiredChanges, tg)
	} else {
		desiredChanges.Ignore += uint64(len(destructive))
	}

	a.computeMigrations(desiredChanges, migrate, tg, isCanarying)
	a.createDeployment(tg.Name, tg.Update, existingDeployment, dstate, all, destructive)

	// Deployments that are still initializing need to be sent in full in the
	// plan so its internal state can be persisted by the plan applier.
	if a.deployment != nil && a.deployment.Status == structs.DeploymentStatusInitializing {
		a.result.deployment = a.deployment
	}

	deploymentComplete := a.isDeploymentComplete(groupName, destructive, inplace,
		migrate, rescheduleNow, place, rescheduleLater, requiresCanaries)

	return deploymentComplete
}

func (a *allocReconciler) initializeDeploymentState(group string, tg *structs.TaskGroup) (*structs.DeploymentState, bool) {
	var dstate *structs.DeploymentState
	existingDeployment := false

	if a.deployment != nil {
		dstate, existingDeployment = a.deployment.TaskGroups[group]
	}

	if !existingDeployment {
		dstate = &structs.DeploymentState{}
		if !tg.Update.IsEmpty() {
			dstate.AutoRevert = tg.Update.AutoRevert
			dstate.AutoPromote = tg.Update.AutoPromote
			dstate.ProgressDeadline = tg.Update.ProgressDeadline
		}
	}

	return dstate, existingDeployment
}

// If we have destructive updates, and have fewer canaries than is desired, we need to create canaries.
func (a *allocReconciler) requiresCanaries(tg *structs.TaskGroup, dstate *structs.DeploymentState, destructive, canaries alloc.Set) bool {
	canariesPromoted := dstate != nil && dstate.Promoted
	return tg.Update != nil &&
		len(destructive) != 0 &&
		len(canaries) < tg.Update.Canary &&
		!canariesPromoted
}

func (a *allocReconciler) computeCanaries(tg *structs.TaskGroup, dstate *structs.DeploymentState,
	destructive, canaries alloc.Set, desiredChanges *structs.DesiredUpdates, nameIndex *alloc.NameIndex) {
	dstate.DesiredCanaries = tg.Update.Canary

	if !a.deploymentPaused && !a.deploymentFailed {
		desiredChanges.Canary += uint64(tg.Update.Canary - len(canaries))
		for _, name := range nameIndex.NextCanaries(uint(desiredChanges.Canary), canaries, destructive) {
			a.result.place = append(a.result.place, allocPlaceResult{
				name:      name,
				canary:    true,
				taskGroup: tg,
			})
		}
	}
}

// filterOldTerminalAllocs filters allocations that should be ignored since they
// are allocations that are terminal from a previous job version.
func (a *allocReconciler) filterOldTerminalAllocs(all alloc.Set) (filtered, ignore alloc.Set) {
	if !a.batch {
		return all, nil
	}

	filtered = filtered.Union(all)
	ignored := make(map[string]*structs.Allocation)

	// Ignore terminal batch jobs from older versions
	for id, filteredAlloc := range filtered {
		older := filteredAlloc.Job.Version < a.job.Version || filteredAlloc.Job.CreateIndex < a.job.CreateIndex
		if older && filteredAlloc.TerminalStatus() {
			delete(filtered, id)
			ignored[id] = filteredAlloc
		}
	}

	return filtered, ignored
}

// cancelUnneededCanaries handles the canaries for the group by stopping the
// unneeded ones and returning the current set of canaries and the updated total
// set of allocs for the group
func (a *allocReconciler) cancelUnneededCanaries(original alloc.Set, desiredChanges *structs.DesiredUpdates) (canaries, all alloc.Set) {
	// Stop any canary from an older deployment or from a failed one
	var stop []string

	all = original

	// Cancel any non-promoted canaries from the older deployment
	if a.oldDeployment != nil {
		for _, dstate := range a.oldDeployment.TaskGroups {
			if !dstate.Promoted {
				stop = append(stop, dstate.PlacedCanaries...)
			}
		}
	}

	// Cancel any non-promoted canaries from a failed deployment
	if a.deployment != nil && a.deployment.Status == structs.DeploymentStatusFailed {
		for _, dstate := range a.deployment.TaskGroups {
			if !dstate.Promoted {
				stop = append(stop, dstate.PlacedCanaries...)
			}
		}
	}

	// stopSet is the allocSet that contains the canaries we desire to stop from
	// above.
	stopSet := all.FromKeys(stop)
	a.markStop(stopSet, "", allocNotNeeded)
	desiredChanges.Stop += uint64(len(stopSet))
	all = all.Difference(stopSet)

	// Capture our current set of canaries and handle any migrations that are
	// needed by just stopping them.
	if a.deployment != nil {
		var canaryIDs []string
		for _, dstate := range a.deployment.TaskGroups {
			canaryIDs = append(canaryIDs, dstate.PlacedCanaries...)
		}

		canaries = all.FromKeys(canaryIDs)
		untainted, migrate, lost, _, _, _ := canaries.FilterByTainted(a.taintedNodes, a.supportsDisconnectedClients, a.now)
		// We don't add these stops to desiredChanges because the deployment is
		// still active. DesiredChanges is used to report deployment progress/final
		// state. These transient failures aren't meaningful.
		a.markStop(migrate, "", allocMigrating)
		a.markStop(lost, structs.AllocClientStatusLost, allocLost)

		canaries = untainted
		all = all.Difference(migrate, lost)
	}

	return
}

// computeUnderProvisionedBy returns the number of allocs that still need to be
// placed for a particular group. The inputs are the group definition, the untainted,
// destructive, and migrate allocation sets, and whether we are in a canary state.
func (a *allocReconciler) computeUnderProvisionedBy(group *structs.TaskGroup, untainted, destructive, migrate alloc.Set, isCanarying bool) int {
	// If no update strategy, nothing is migrating, and nothing is being replaced,
	// allow as many as defined in group.Count
	if group.Update.IsEmpty() || len(destructive)+len(migrate) == 0 {
		return group.Count
	}

	// If the deployment is nil, allow MaxParallel placements
	if a.deployment == nil {
		return group.Update.MaxParallel
	}

	// If the deployment is paused, failed, or we have un-promoted canaries, do not create anything else.
	if a.deploymentPaused ||
		a.deploymentFailed ||
		isCanarying {
		return 0
	}

	underProvisionedBy := group.Update.MaxParallel
	partOf, _ := untainted.FilterByDeployment(a.deployment.ID)
	for _, deploymentAlloc := range partOf {
		// An unhealthy allocation means nothing else should happen.
		if deploymentAlloc.DeploymentStatus.IsUnhealthy() {
			return 0
		}
		// If not yet explicitly set to healthy (nil) decrement.
		if !deploymentAlloc.DeploymentStatus.IsHealthy() {
			underProvisionedBy--
		}
	}

	// The limit can be less than zero in the case that the job was changed such
	// that it required destructive changes and the count was scaled up.
	if underProvisionedBy < 0 {
		return 0
	}

	return underProvisionedBy
}

// computePlacements returns the set of allocations to place given the group
// definition, the set of untainted, migrating and reschedule allocations for the group.
//
// Placements will meet or exceed group count.
func (a *allocReconciler) computePlacements(group *structs.TaskGroup,
	nameIndex *alloc.NameIndex, untainted, migrate, reschedule, lost alloc.Set,
	isCanarying bool) []allocPlaceResult {

	// Add rescheduled placement results
	var place []allocPlaceResult
	for _, rescheduleAlloc := range reschedule {
		place = append(place, allocPlaceResult{
			name:          rescheduleAlloc.Name,
			taskGroup:     group,
			previousAlloc: rescheduleAlloc,
			reschedule:    true,
			canary:        rescheduleAlloc.DeploymentStatus.IsCanary(),

			downgradeNonCanary: isCanarying && !rescheduleAlloc.DeploymentStatus.IsCanary(),
			minJobVersion:      rescheduleAlloc.Job.Version,
			lost:               false,
		})
	}

	// Add replacements for disconnected and lost allocs up to group.Count
	existing := len(untainted) + len(migrate) + len(reschedule)

	// Add replacements for lost
	for _, lostAlloc := range lost {
		if existing >= group.Count {
			// Reached desired count, do not replace remaining lost
			// allocs
			break
		}

		existing++
		place = append(place, allocPlaceResult{
			name:               lostAlloc.Name,
			taskGroup:          group,
			previousAlloc:      lostAlloc,
			reschedule:         false,
			canary:             lostAlloc.DeploymentStatus.IsCanary(),
			downgradeNonCanary: isCanarying && !lostAlloc.DeploymentStatus.IsCanary(),
			minJobVersion:      lostAlloc.Job.Version,
			lost:               true,
		})
	}

	// Add remaining placement results
	if existing < group.Count {
		for _, name := range nameIndex.Next(uint(group.Count - existing)) {
			place = append(place, allocPlaceResult{
				name:               name,
				taskGroup:          group,
				downgradeNonCanary: isCanarying,
			})
		}
	}

	return place
}

// computeReplacements either applies the placements calculated by computePlacements,
// or computes more placements based on whether the deployment is ready for placement
// and if the placement is already rescheduling or part of a failed deployment.
// The input deploymentPlaceReady is calculated as the deployment is not paused, failed, or canarying.
// It returns the number of allocs still needed.
func (a *allocReconciler) computeReplacements(deploymentPlaceReady bool, desiredChanges *structs.DesiredUpdates,
	place []allocPlaceResult, rescheduleNow, lost alloc.Set, underProvisionedBy int) int {

	// Disconnecting allocs are not failing, but are included in rescheduleNow.
	// Create a new set that only includes the actual failures and compute
	// replacements based off that.
	failed := make(alloc.Set)
	for id, rescheduleNowAlloc := range rescheduleNow {
		_, ok := a.result.disconnectUpdates[id]
		if !ok && rescheduleNowAlloc.ClientStatus != structs.AllocClientStatusUnknown {
			failed[id] = rescheduleNowAlloc
		}
	}

	// If the deployment is place ready, apply all placements and return
	if deploymentPlaceReady {
		desiredChanges.Place += uint64(len(place))
		// This relies on the computePlacements having built this set, which in
		// turn relies on len(lostLater) == 0.
		a.result.place = append(a.result.place, place...)

		a.markStop(failed, "", allocRescheduled)
		desiredChanges.Stop += uint64(len(failed))

		minimum := min(len(place), underProvisionedBy)
		underProvisionedBy -= minimum
		return underProvisionedBy
	}

	// We do not want to place additional allocations but in the case we
	// have lost allocations or allocations that require rescheduling now,
	// we do so regardless to avoid odd user experiences.

	// If allocs have been lost, determine the number of replacements that are needed
	// and add placements to the result for the lost allocs.
	if len(lost) != 0 {
		allowed := min(len(lost), len(place))
		desiredChanges.Place += uint64(allowed)
		a.result.place = append(a.result.place, place[:allowed]...)
	}

	// if no failures or there are no pending placements return.
	if len(rescheduleNow) == 0 || len(place) == 0 {
		return underProvisionedBy
	}

	// Handle rescheduling of failed allocations even if the deployment is failed.
	// If the placement is rescheduling, and not part of a failed deployment, add
	// to the place set. Add the previous alloc to the stop set unless it is disconnecting.
	for _, p := range place {
		prev := p.PreviousAllocation()
		partOfFailedDeployment := a.deploymentFailed && prev != nil && a.deployment.ID == prev.DeploymentID

		if !partOfFailedDeployment && p.IsRescheduling() {
			a.result.place = append(a.result.place, p)
			desiredChanges.Place++

			_, prevIsDisconnecting := a.result.disconnectUpdates[prev.ID]
			if prevIsDisconnecting {
				continue
			}

			a.result.stop = append(a.result.stop, allocStopResult{
				alloc:             prev,
				statusDescription: allocRescheduled,
			})
			desiredChanges.Stop++
		}
	}

	return underProvisionedBy
}

func (a *allocReconciler) computeDestructiveUpdates(destructive alloc.Set, underProvisionedBy int,
	desiredChanges *structs.DesiredUpdates, tg *structs.TaskGroup) {

	// Do all destructive updates
	minimum := min(len(destructive), underProvisionedBy)
	desiredChanges.DestructiveUpdate += uint64(minimum)
	desiredChanges.Ignore += uint64(len(destructive) - minimum)
	for _, destructiveAlloc := range destructive.NameOrder()[:minimum] {
		a.result.destructiveUpdate = append(a.result.destructiveUpdate, allocDestructiveResult{
			placeName:             destructiveAlloc.Name,
			placeTaskGroup:        tg,
			stopAlloc:             destructiveAlloc,
			stopStatusDescription: allocUpdating,
		})
	}
}

func (a *allocReconciler) computeMigrations(desiredChanges *structs.DesiredUpdates, migrate alloc.Set, tg *structs.TaskGroup, isCanarying bool) {
	desiredChanges.Migrate += uint64(len(migrate))
	for _, migrateAlloc := range migrate.NameOrder() {
		a.result.stop = append(a.result.stop, allocStopResult{
			alloc:             migrateAlloc,
			statusDescription: allocMigrating,
		})
		a.result.place = append(a.result.place, allocPlaceResult{
			name:          migrateAlloc.Name,
			canary:        migrateAlloc.DeploymentStatus.IsCanary(),
			taskGroup:     tg,
			previousAlloc: migrateAlloc,

			downgradeNonCanary: isCanarying && !migrateAlloc.DeploymentStatus.IsCanary(),
			minJobVersion:      migrateAlloc.Job.Version,
		})
	}
}

func (a *allocReconciler) createDeployment(groupName string, strategy *structs.UpdateStrategy,
	existingDeployment bool, dstate *structs.DeploymentState, all, destructive alloc.Set) {
	// Guard the simple cases that require no computation first.
	if existingDeployment ||
		strategy.IsEmpty() ||
		dstate.DesiredTotal == 0 {
		return
	}

	updatingSpec := len(destructive) != 0 || len(a.result.inplaceUpdate) != 0

	hadRunning := false
	for _, allAlloc := range all {
		if allAlloc.Job.Version == a.job.Version && allAlloc.Job.CreateIndex == a.job.CreateIndex {
			hadRunning = true
			break
		}
	}

	// Don't create a deployment if it's not the first time running the job
	// and there are no updates to the spec.
	if hadRunning && !updatingSpec {
		return
	}

	// A previous group may have made the deployment already. If not create one.
	if a.deployment == nil {
		a.deployment = structs.NewDeployment(a.job, a.evalPriority)
		a.result.deployment = a.deployment
	}

	// Attach the groups deployment state to the deployment
	a.deployment.TaskGroups[groupName] = dstate
}

func (a *allocReconciler) isDeploymentComplete(groupName string, destructive, inplace, migrate, rescheduleNow alloc.Set,
	place []allocPlaceResult, rescheduleLater []*alloc.DelayedRescheduleInfo, requiresCanaries bool) bool {

	complete := len(destructive)+len(inplace)+len(place)+len(migrate)+len(rescheduleNow)+len(rescheduleLater) == 0 &&
		!requiresCanaries

	if !complete || a.deployment == nil {
		return false
	}

	// Final check to see if the deployment is complete is to ensure everything is healthy
	if dstate, ok := a.deployment.TaskGroups[groupName]; ok {
		if dstate.HealthyAllocs < max(dstate.DesiredTotal, dstate.DesiredCanaries) || // Make sure we have enough healthy allocs
			(dstate.DesiredCanaries > 0 && !dstate.Promoted) { // Make sure we are promoted if we have canaries
			complete = false
		}
	}

	return complete
}

// computeStop returns the set of allocations that are marked for stopping given
// the group definition, the set of allocations in various states and whether we
// are canarying.
func (a *allocReconciler) computeStop(group *structs.TaskGroup, nameIndex *alloc.NameIndex,
	untainted, migrate, lost, canaries alloc.Set, isCanarying bool, followupEvals map[string]string) alloc.Set {

	// Mark all lost allocations for stop.
	var stop alloc.Set
	stop = stop.Union(lost)
	a.markDelayed(lost, structs.AllocClientStatusLost, allocLost, followupEvals)

	// If we are still deploying or creating canaries, don't stop them
	if isCanarying {
		untainted = untainted.Difference(canaries)
	}

	// Remove disconnected allocations so they won't be stopped
	knownUntainted := untainted.FilterOutByClientStatus(structs.AllocClientStatusUnknown)

	// Hot path the nothing to do case
	//
	// Note that this path can result in duplication allocation indexes in a
	// scenario with a destructive job change (ex. image update) happens with
	// an increased group count. Once the canary is replaced, and we compute
	// the next set of stops, the untainted set equals the new group count,
	// which results is missing one removal. The duplicate alloc index is
	// corrected in `computePlacements`
	remove := len(knownUntainted) + len(migrate) - group.Count
	if remove <= 0 {
		return stop
	}

	// Filter out any terminal allocations from the untainted set
	// This is so that we don't try to mark them as stopped redundantly
	untainted = alloc.FilterByTerminal(untainted)

	// Prefer stopping any alloc that has the same name as the canaries if we
	// are promoted
	if !isCanarying && len(canaries) != 0 {
		canaryNames := canaries.NameSet()
		for id, untaintedAlloc := range untainted.Difference(canaries) {
			if _, match := canaryNames[untaintedAlloc.Name]; match {
				stop[id] = untaintedAlloc
				a.result.stop = append(a.result.stop, allocStopResult{
					alloc:             untaintedAlloc,
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
		migratingNames := alloc.NewNameIndex(a.jobID, group.Name, group.Count, migrate)
		removeNames := migratingNames.Highest(uint(remove))
		for id, migrateAlloc := range migrate {
			if _, match := removeNames[migrateAlloc.Name]; !match {
				continue
			}
			a.result.stop = append(a.result.stop, allocStopResult{
				alloc:             migrateAlloc,
				statusDescription: allocNotNeeded,
			})
			delete(migrate, id)
			stop[id] = migrateAlloc
			nameIndex.UnsetIndex(migrateAlloc.Index())

			remove--
			if remove == 0 {
				return stop
			}
		}
	}

	// Select the allocs with the highest count to remove
	removeNames := nameIndex.Highest(uint(remove))
	for id, untaintedAlloc := range untainted {
		if _, ok := removeNames[untaintedAlloc.Name]; ok {
			stop[id] = untaintedAlloc
			a.result.stop = append(a.result.stop, allocStopResult{
				alloc:             untaintedAlloc,
				statusDescription: allocNotNeeded,
			})
			delete(untainted, id)

			remove--
			if remove == 0 {
				return stop
			}
		}
	}

	// It is possible that we didn't stop as many as we should have if there
	// were allocations with duplicate names.
	for id, untaintedAlloc := range untainted {
		stop[id] = untaintedAlloc
		a.result.stop = append(a.result.stop, allocStopResult{
			alloc:             untaintedAlloc,
			statusDescription: allocNotNeeded,
		})
		delete(untainted, id)

		remove--
		if remove == 0 {
			return stop
		}
	}

	return stop
}

// reconcileReconnecting receives the set of allocations that are reconnecting
// and all other allocations for the same group and determines which ones to
// reconnect which ones or stop.
//
//   - Every reconnecting allocation MUST be present in one, and only one, of
//     the returned set.
//   - Every replacement allocation that is not preferred MUST be returned in
//     the stop set.
//   - Only reconnecting allocations are allowed to be present in the returned
//     reconnect set.
//   - If the reconnecting allocation is to be stopped, its replacements may
//     not be present in any of the returned sets. The rest of the reconciler
//     logic will handle them.
func (a *allocReconciler) reconcileReconnecting(reconnecting, all alloc.Set) (alloc.Set, alloc.Set) {
	stop := make(alloc.Set)
	reconnect := make(alloc.Set)

	for _, reconnectingAlloc := range reconnecting {
		// Stop allocations that failed to reconnect.
		reconnectFailed := !reconnectingAlloc.ServerTerminalStatus() &&
			reconnectingAlloc.ClientStatus == structs.AllocClientStatusFailed

		if reconnectFailed {
			stop[reconnectingAlloc.ID] = reconnectingAlloc
			a.result.stop = append(a.result.stop, allocStopResult{
				alloc:             reconnectingAlloc,
				clientStatus:      structs.AllocClientStatusFailed,
				statusDescription: allocRescheduled,
			})
			continue
		}

		// If the desired status is not run, or if the user-specified desired
		// transition is not run, stop the reconnecting allocation.
		stopReconnecting := reconnectingAlloc.DesiredStatus != structs.AllocDesiredStatusRun ||
			reconnectingAlloc.DesiredTransition.ShouldMigrate() ||
			reconnectingAlloc.DesiredTransition.ShouldReschedule() ||
			reconnectingAlloc.DesiredTransition.ShouldForceReschedule() ||
			reconnectingAlloc.Job.Version < a.job.Version ||
			reconnectingAlloc.Job.CreateIndex < a.job.CreateIndex

		if stopReconnecting {
			stop[reconnectingAlloc.ID] = reconnectingAlloc
			a.result.stop = append(a.result.stop, allocStopResult{
				alloc:             reconnectingAlloc,
				statusDescription: allocNotNeeded,
			})
			continue
		}

		// Find replacement allocations and decide which one to stop. A
		// reconnecting allocation may have multiple replacements.
		for _, replacementAlloc := range all {

			// Skip allocations that are not a replacement of the one
			// reconnecting.
			isReplacement := replacementAlloc.ID == reconnectingAlloc.NextAllocation

			// Skip allocations that are server terminal.
			// We don't want to replace a reconnecting allocation with one that
			// is or will terminate and we don't need to stop them since they
			// are already marked as terminal by the servers.
			if !isReplacement || replacementAlloc.ServerTerminalStatus() {
				continue
			}

			// Pick which allocation we want to keep.
			keepAlloc := pickReconnectingAlloc(reconnectingAlloc, replacementAlloc)
			if keepAlloc == replacementAlloc {
				// The replacement allocation is preferred, so stop the one
				// reconnecting if not stopped yet.
				if _, ok := stop[reconnectingAlloc.ID]; !ok {
					stop[reconnectingAlloc.ID] = reconnectingAlloc
					a.result.stop = append(a.result.stop, allocStopResult{
						alloc:             reconnectingAlloc,
						statusDescription: allocNotNeeded,
					})
				}
			} else {
				// The reconnecting allocation is preferred, so stop this
				// replacement, but avoid re-stopping stopped allocs
				if replacementAlloc.ClientStatus != structs.AllocClientStatusFailed {
					stop[replacementAlloc.ID] = replacementAlloc
					a.result.stop = append(a.result.stop, allocStopResult{
						alloc:             replacementAlloc,
						statusDescription: allocReconnected,
					})
				}
			}
		}
	}

	// Any reconnecting allocation not set to stop must be reconnected.
	for _, reconnectingAlloc := range reconnecting {
		if _, ok := stop[reconnectingAlloc.ID]; !ok {
			reconnect[reconnectingAlloc.ID] = reconnectingAlloc
		}
	}

	return reconnect, stop
}

// pickReconnectingAlloc returns the allocation to keep between the original
// one that is reconnecting and one of its replacements.
//
// This function is not commutative, meaning that pickReconnectingAlloc(A, B)
// is not the same as pickReconnectingAlloc(B, A). Preference is given to keep
// the original allocation when possible.
func pickReconnectingAlloc(original *structs.Allocation, replacement *structs.Allocation) *structs.Allocation {
	// Check if the replacement is newer.
	// Always prefer the replacement if true.
	replacementIsNewer := replacement.Job.Version > original.Job.Version ||
		replacement.Job.CreateIndex > original.Job.CreateIndex
	if replacementIsNewer {
		return replacement
	}

	// Check if the replacement has better placement score.
	// If any of the scores is not available, only pick the replacement if
	// itself does have scores.
	originalMaxScoreMeta := original.Metrics.MaxNormScore()
	replacementMaxScoreMeta := replacement.Metrics.MaxNormScore()

	replacementHasBetterScore := originalMaxScoreMeta == nil && replacementMaxScoreMeta != nil ||
		(originalMaxScoreMeta != nil && replacementMaxScoreMeta != nil &&
			replacementMaxScoreMeta.NormScore > originalMaxScoreMeta.NormScore)

	// Check if the replacement has better client status.
	// Even with a better placement score make sure we don't replace a running
	// allocation with one that is not.
	replacementIsRunning := replacement.ClientStatus == structs.AllocClientStatusRunning
	originalNotRunning := original.ClientStatus != structs.AllocClientStatusRunning

	if replacementHasBetterScore && (replacementIsRunning || originalNotRunning) {
		return replacement
	}

	return original
}

// computeUpdates determines which allocations for the passed group require
// updates. Three groups are returned:
// 1. Those that require no upgrades
// 2. Those that can be upgraded in-place. These are added to the results
// automatically since the function contains the correct state to do so,
// 3. Those that require destructive updates
func (a *allocReconciler) computeUpdates(group *structs.TaskGroup, untainted alloc.Set) (ignore, inplace, destructive alloc.Set) {
	// Determine the set of allocations that need to be updated
	ignore = make(map[string]*structs.Allocation)
	inplace = make(map[string]*structs.Allocation)
	destructive = make(map[string]*structs.Allocation)

	for _, untainedAlloc := range untainted {
		ignoreChange, destructiveChange, inplaceAlloc := a.allocUpdateFn(untainedAlloc, a.job, group)
		if ignoreChange {
			ignore[untainedAlloc.ID] = untainedAlloc
		} else if destructiveChange {
			destructive[untainedAlloc.ID] = untainedAlloc
		} else {
			inplace[untainedAlloc.ID] = untainedAlloc
			a.result.inplaceUpdate = append(a.result.inplaceUpdate, inplaceAlloc)
		}
	}

	return
}

// createRescheduleLaterEvals creates batched followup evaluations with the WaitUntil field
// set for allocations that are eligible to be rescheduled later, and marks the alloc with
// the followupEvalID
func (a *allocReconciler) createRescheduleLaterEvals(rescheduleLater []*alloc.DelayedRescheduleInfo, all alloc.Set, tgName string) {
	// followupEvals are created in the same way as for delayed lost allocs
	allocIDToFollowupEvalID := a.createLostLaterEvals(rescheduleLater, tgName)

	// Create updates that will be applied to the allocs to mark the FollowupEvalID
	for _, laterAlloc := range rescheduleLater {
		existingAlloc := all[laterAlloc.Alloc.ID]
		updatedAlloc := existingAlloc.Copy()
		updatedAlloc.FollowupEvalID = allocIDToFollowupEvalID[laterAlloc.Alloc.ID]

		// Can't updated an allocation that is disconnected
		if _, ok := a.result.disconnectUpdates[laterAlloc.AllocID]; !ok {
			a.result.attributeUpdates[laterAlloc.AllocID] = updatedAlloc
		} else {
			a.result.disconnectUpdates[laterAlloc.AllocID].FollowupEvalID = allocIDToFollowupEvalID[laterAlloc.Alloc.ID]
		}
	}
}

// computeReconnecting copies existing allocations in the unknown state, but
// whose nodes have been identified as ready. The Allocations DesiredStatus is
// set to running, and these allocs are appended to the Plan as non-destructive
// updates. Clients are responsible for reconciling the DesiredState with the
// actual state as the node comes back online.
func (a *allocReconciler) computeReconnecting(reconnecting alloc.Set) {
	if len(reconnecting) == 0 {
		return
	}

	// Create updates that will be appended to the plan.
	for _, reconnectingAlloc := range reconnecting {
		// If the user has defined a DesiredTransition don't resume the alloc.
		if reconnectingAlloc.DesiredTransition.ShouldMigrate() ||
			reconnectingAlloc.DesiredTransition.ShouldReschedule() ||
			reconnectingAlloc.DesiredTransition.ShouldForceReschedule() ||
			reconnectingAlloc.Job.Version < a.job.Version ||
			reconnectingAlloc.Job.CreateIndex < a.job.CreateIndex {
			continue
		}

		// If the scheduler has defined a terminal DesiredStatus don't resume the alloc.
		if reconnectingAlloc.DesiredStatus != structs.AllocDesiredStatusRun {
			continue
		}

		// If the alloc has failed don't reconnect.
		if reconnectingAlloc.ClientStatus != structs.AllocClientStatusRunning {
			continue
		}

		// Record the new ClientStatus to indicate to future evals that the
		// alloc has already reconnected.
		// Use a copy to prevent mutating the object from statestore.
		reconnectedAlloc := reconnectingAlloc.Copy()
		reconnectedAlloc.AppendState(structs.AllocStateFieldClientStatus, reconnectingAlloc.ClientStatus)
		a.result.reconnectUpdates[reconnectedAlloc.ID] = reconnectedAlloc
	}
}

// handleDelayedLost creates batched followup evaluations with the WaitUntil field set for
// lost allocations. followupEvals are appended to a.result as a side effect, we return a
// map of alloc IDs to their followupEval IDs.
func (a *allocReconciler) createLostLaterEvals(rescheduleLater []*alloc.DelayedRescheduleInfo, tgName string) map[string]string {
	if len(rescheduleLater) == 0 {
		return map[string]string{}
	}

	// Sort by time
	sort.Slice(rescheduleLater, func(i, j int) bool {
		return rescheduleLater[i].RescheduleTime.Before(rescheduleLater[j].RescheduleTime)
	})

	var evals []*structs.Evaluation
	nextReschedTime := rescheduleLater[0].RescheduleTime
	allocIDToFollowupEvalID := make(map[string]string, len(rescheduleLater))

	// Create a new eval for the first batch
	eval := &structs.Evaluation{
		ID:                uuid.Generate(),
		Namespace:         a.job.Namespace,
		Priority:          a.evalPriority,
		Type:              a.job.Type,
		TriggeredBy:       structs.EvalTriggerRetryFailedAlloc,
		JobID:             a.job.ID,
		JobModifyIndex:    a.job.ModifyIndex,
		Status:            structs.EvalStatusPending,
		StatusDescription: reschedulingFollowupEvalDesc,
		WaitUntil:         nextReschedTime,
	}
	evals = append(evals, eval)

	for _, allocReschedInfo := range rescheduleLater {
		if allocReschedInfo.RescheduleTime.Sub(nextReschedTime) < batchedFailedAllocWindowSize {
			allocIDToFollowupEvalID[allocReschedInfo.AllocID] = eval.ID
		} else {
			// Start a new batch
			nextReschedTime = allocReschedInfo.RescheduleTime
			// Create a new eval for the new batch
			eval = &structs.Evaluation{
				ID:             uuid.Generate(),
				Namespace:      a.job.Namespace,
				Priority:       a.evalPriority,
				Type:           a.job.Type,
				TriggeredBy:    structs.EvalTriggerRetryFailedAlloc,
				JobID:          a.job.ID,
				JobModifyIndex: a.job.ModifyIndex,
				Status:         structs.EvalStatusPending,
				WaitUntil:      nextReschedTime,
			}
			evals = append(evals, eval)
			// Set the evalID for the first alloc in this new batch
			allocIDToFollowupEvalID[allocReschedInfo.AllocID] = eval.ID
		}
		emitRescheduleInfo(allocReschedInfo.Alloc, eval)
	}

	a.appendFollowupEvals(tgName, evals)

	return allocIDToFollowupEvalID
}

// createTimeoutLaterEvals creates followup evaluations with the
// WaitUntil field set for allocations in an unknown state on disconnected nodes.
// Followup Evals are appended to a.result as a side effect. It returns a map of
// allocIDs to their associated followUpEvalIDs.
func (a *allocReconciler) createTimeoutLaterEvals(disconnecting alloc.Set, tgName string) map[string]string {
	if len(disconnecting) == 0 {
		return map[string]string{}
	}

	timeoutDelays, err := disconnecting.DelayByMaxClientDisconnect(a.now)
	if err != nil {
		a.logger.Error("error for task_group",
			"task_group", tgName, "error", err)
		return map[string]string{}
	}

	// Sort by time
	sort.Slice(timeoutDelays, func(i, j int) bool {
		return timeoutDelays[i].RescheduleTime.Before(timeoutDelays[j].RescheduleTime)
	})

	var evals []*structs.Evaluation
	nextReschedTime := timeoutDelays[0].RescheduleTime
	allocIDToFollowupEvalID := make(map[string]string, len(timeoutDelays))

	eval := &structs.Evaluation{
		ID:                uuid.Generate(),
		Namespace:         a.job.Namespace,
		Priority:          a.evalPriority,
		Type:              a.job.Type,
		TriggeredBy:       structs.EvalTriggerMaxDisconnectTimeout,
		JobID:             a.job.ID,
		JobModifyIndex:    a.job.ModifyIndex,
		Status:            structs.EvalStatusPending,
		StatusDescription: disconnectTimeoutFollowupEvalDesc,
		WaitUntil:         nextReschedTime,
	}
	evals = append(evals, eval)

	// Important to remember that these are sorted. The rescheduleTime can only
	// get farther into the future. If this loop detects the next delay is greater
	// than the batch window (5s) it creates another batch.
	for _, timeoutInfo := range timeoutDelays {
		if timeoutInfo.RescheduleTime.Sub(nextReschedTime) < batchedFailedAllocWindowSize {
			allocIDToFollowupEvalID[timeoutInfo.AllocID] = eval.ID
		} else {
			// Start a new batch
			nextReschedTime = timeoutInfo.RescheduleTime
			// Create a new eval for the new batch
			eval = &structs.Evaluation{
				ID:                uuid.Generate(),
				Namespace:         a.job.Namespace,
				Priority:          a.evalPriority,
				Type:              a.job.Type,
				TriggeredBy:       structs.EvalTriggerMaxDisconnectTimeout,
				JobID:             a.job.ID,
				JobModifyIndex:    a.job.ModifyIndex,
				Status:            structs.EvalStatusPending,
				StatusDescription: disconnectTimeoutFollowupEvalDesc,
				WaitUntil:         timeoutInfo.RescheduleTime,
			}
			evals = append(evals, eval)
			allocIDToFollowupEvalID[timeoutInfo.AllocID] = eval.ID
		}

		emitRescheduleInfo(timeoutInfo.Alloc, eval)

		// Create updates that will be applied to the allocs to mark the FollowupEvalID
		// and the unknown ClientStatus and AllocState.
		updatedAlloc := timeoutInfo.Alloc.Copy()
		updatedAlloc.ClientStatus = structs.AllocClientStatusUnknown
		updatedAlloc.AppendState(structs.AllocStateFieldClientStatus, structs.AllocClientStatusUnknown)
		updatedAlloc.ClientDescription = allocUnknown
		updatedAlloc.FollowupEvalID = eval.ID
		a.result.disconnectUpdates[updatedAlloc.ID] = updatedAlloc
	}

	a.appendFollowupEvals(tgName, evals)

	return allocIDToFollowupEvalID
}

// appendFollowupEvals appends a set of followup evals for a task group to the
// desiredFollowupEvals map which is later added to the scheduler's followUpEvals set.
func (a *allocReconciler) appendFollowupEvals(tgName string, evals []*structs.Evaluation) {
	// Merge with
	if existingFollowUpEvals, ok := a.result.desiredFollowupEvals[tgName]; ok {
		evals = append(existingFollowUpEvals, evals...)
	}

	a.result.desiredFollowupEvals[tgName] = evals
}

// emitRescheduleInfo emits metrics about the rescheduling decision of an evaluation. If a followup evaluation is
// provided, the waitUntil time is emitted.
func emitRescheduleInfo(alloc *structs.Allocation, followupEval *structs.Evaluation) {
	// Emit short-lived metrics data point. Note, these expire and stop emitting after about a minute.
	baseMetric := []string{"scheduler", "allocs", "reschedule"}
	labels := []metrics.Label{
		{Name: "alloc_id", Value: alloc.ID},
		{Name: "job", Value: alloc.JobID},
		{Name: "namespace", Value: alloc.Namespace},
		{Name: "task_group", Value: alloc.TaskGroup},
	}
	if followupEval != nil {
		labels = append(labels, metrics.Label{Name: "followup_eval_id", Value: followupEval.ID})
		metrics.SetGaugeWithLabels(append(baseMetric, "wait_until"), float32(followupEval.WaitUntil.Unix()), labels)
	}
	attempted, availableAttempts := alloc.RescheduleInfo()
	metrics.SetGaugeWithLabels(append(baseMetric, "attempted"), float32(attempted), labels)
	metrics.SetGaugeWithLabels(append(baseMetric, "limit"), float32(availableAttempts), labels)
}
