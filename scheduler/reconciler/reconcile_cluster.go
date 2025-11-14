// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

// The reconciler is the first stage in the scheduler for service and batch
// jobs. It compares the existing state to the desired state to determine the
// set of changes needed. System jobs and sysbatch jobs do not use the
// reconciler.

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"time"

	log "github.com/hashicorp/go-hclog"
	metrics "github.com/hashicorp/go-metrics/compat"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

const (
	// batchedFailedAllocWindowSize is the window size used
	// to batch up failed allocations before creating an eval
	batchedFailedAllocWindowSize = 5 * time.Second

	// rescheduleWindowSize is the window size relative to
	// current time within which reschedulable allocations are placed.
	// This helps protect against small clock drifts between servers
	rescheduleWindowSize = 1 * time.Second
)

// AllocUpdateType takes an existing allocation and a new job definition and
// returns whether the allocation can ignore the change, requires a destructive
// update, or can be inplace updated. If it can be inplace updated, an updated
// allocation that has the new resources and alloc metrics attached will be
// returned.
type AllocUpdateType func(existing *structs.Allocation, newJob *structs.Job,
	newTG *structs.TaskGroup) (ignore, destructive bool, updated *structs.Allocation)

type AllocReconcilerOption func(*AllocReconciler)

// ReconcilerState holds initial and intermittent state of the reconciler
type ReconcilerState struct {
	Job        *structs.Job
	JobID      string // stored separately because the job can be nil
	JobIsBatch bool

	DeploymentOld     *structs.Deployment
	DeploymentCurrent *structs.Deployment
	DeploymentPaused  bool
	DeploymentFailed  bool

	ExistingAllocs []*structs.Allocation

	EvalID       string
	EvalPriority int
}

// AllocReconciler is used to determine the set of allocations that require
// placement, inplace updating or stopping given the job specification and
// existing cluster state. The reconciler should only be used for batch and
// service jobs.
type AllocReconciler struct {
	// logger is used to log debug information. Logging should be kept at a
	// minimal here
	logger log.Logger

	// canInplace is used to check if the allocation can be inplace upgraded
	allocUpdateFn AllocUpdateType

	// jobState holds information about job, deployment, allocs and eval
	jobState ReconcilerState

	reconnectingPicker reconnectingPickerInterface

	// clusterState stores frequently accessed properties of the cluster:
	// - a map of tainted nodes
	// - whether we support disconnected clients
	// - current time
	clusterState ClusterState
}

// ReconcileResults contains the results of the reconciliation and should be
// applied by the scheduler.
type ReconcileResults struct {
	// Deployment is the Deployment that should be created or updated as a
	// result of scheduling
	Deployment *structs.Deployment

	// DeploymentUpdates contains a set of deployment updates that should be
	// applied as a result of scheduling
	DeploymentUpdates []*structs.DeploymentStatusUpdate

	// Place is the set of allocations to Place by the scheduler
	Place []AllocPlaceResult

	// DestructiveUpdate is the set of allocations to apply a destructive update to
	DestructiveUpdate []allocDestructiveResult

	// InplaceUpdate is the set of allocations to apply an inplace update to
	InplaceUpdate []*structs.Allocation

	// Stop is the set of allocations to Stop
	Stop []AllocStopResult

	// AttributeUpdates are updates to the allocation that are not from a
	// jobspec change.
	AttributeUpdates allocSet

	// DisconnectUpdates is the set of allocations are on disconnected nodes, but
	// have not yet had their ClientStatus set to AllocClientStatusUnknown.
	DisconnectUpdates allocSet

	// ReconnectUpdates is the set of allocations that have ClientStatus set to
	// AllocClientStatusUnknown, but the associated Node has reconnected.
	ReconnectUpdates allocSet

	// DesiredTGUpdates captures the desired set of changes to make for each
	// task group.
	DesiredTGUpdates map[string]*structs.DesiredUpdates

	// DesiredFollowupEvals is the map of follow up evaluations to create per task group
	// This is used to create a delayed evaluation for rescheduling failed allocations.
	DesiredFollowupEvals map[string][]*structs.Evaluation

	// TaskGroupAllocNameIndexes is a tracking of the allocation name index,
	// keyed by the task group name. This is stored within the results, so the
	// generic scheduler can use this to perform duplicate alloc index checks
	// before submitting the plan. This is always non-nil and is handled within
	// a single routine, so does not require a mutex.
	TaskGroupAllocNameIndexes map[string]*AllocNameIndex
}

// Merge merges two instances of ReconcileResults
func (r *ReconcileResults) Merge(new *ReconcileResults) {
	if new.Deployment != nil {
		r.Deployment = new.Deployment
	}
	if new.DeploymentUpdates != nil {
		r.DeploymentUpdates = append(r.DeploymentUpdates, new.DeploymentUpdates...)
	}
	if new.Place != nil {
		r.Place = append(r.Place, new.Place...)
	}
	if new.DestructiveUpdate != nil {
		r.DestructiveUpdate = append(r.DestructiveUpdate, new.DestructiveUpdate...)
	}
	if new.InplaceUpdate != nil {
		r.InplaceUpdate = append(r.InplaceUpdate, new.InplaceUpdate...)
	}
	if new.Stop != nil {
		r.Stop = append(r.Stop, new.Stop...)
	}
	if r.AttributeUpdates != nil {
		maps.Copy(r.AttributeUpdates, new.AttributeUpdates)
	} else {
		r.AttributeUpdates = new.AttributeUpdates
	}
	if r.DisconnectUpdates != nil {
		maps.Copy(r.DisconnectUpdates, new.DisconnectUpdates)
	} else {
		r.DisconnectUpdates = new.DisconnectUpdates
	}
	if r.ReconnectUpdates != nil {
		maps.Copy(r.ReconnectUpdates, new.ReconnectUpdates)
	} else {
		r.ReconnectUpdates = new.ReconnectUpdates
	}
	if r.DesiredTGUpdates != nil {
		maps.Copy(r.DesiredTGUpdates, new.DesiredTGUpdates)
	} else {
		r.DesiredTGUpdates = new.DesiredTGUpdates
	}
	if r.DesiredFollowupEvals != nil {
		maps.Copy(r.DesiredFollowupEvals, new.DesiredFollowupEvals)
	} else {
		r.DesiredFollowupEvals = new.DesiredFollowupEvals
	}
	if r.TaskGroupAllocNameIndexes != nil {
		maps.Copy(r.TaskGroupAllocNameIndexes, new.TaskGroupAllocNameIndexes)
	} else {
		r.TaskGroupAllocNameIndexes = new.TaskGroupAllocNameIndexes
	}
}

// delayedRescheduleInfo contains the allocation id and a time when its eligible to be rescheduled.
// this is used to create follow up evaluations
type delayedRescheduleInfo struct {

	// allocID is the ID of the allocation eligible to be rescheduled
	allocID string

	alloc *structs.Allocation

	// rescheduleTime is the time to use in the delayed evaluation
	rescheduleTime time.Time
}

func (r *ReconcileResults) Fields() []any {
	fields := []any{
		"total_place", len(r.Place),
		"total_destructive", len(r.DestructiveUpdate),
		"total_inplace", len(r.InplaceUpdate),
		"total_stop", len(r.Stop),
		"total_disconnect", len(r.DisconnectUpdates),
		"total_reconnect", len(r.ReconnectUpdates),
	}
	if r.Deployment != nil {
		fields = append(fields, "deployment_created", r.Deployment.ID)
	}
	for _, u := range r.DeploymentUpdates {
		fields = append(fields,
			"deployment_updated", u.DeploymentID,
			"deployment_update", fmt.Sprintf("%s (%s)", u.Status, u.StatusDescription))
	}
	for tg, u := range r.DesiredTGUpdates {
		fields = append(fields,
			tg+"_ignore", u.Ignore,
			tg+"_place", u.Place,
			tg+"_destructive", u.DestructiveUpdate,
			tg+"_inplace", u.InPlaceUpdate,
			tg+"_stop", u.Stop,
			tg+"_migrate", u.Migrate,
			tg+"_canary", u.Canary,
			tg+"_preempt", u.Preemptions,
			tg+"_reschedule_now", u.RescheduleNow,
			tg+"_reschedule_later", u.RescheduleLater,
			tg+"_disconnect", u.Disconnect,
			tg+"_reconnect", u.Reconnect,
		)
	}

	return fields
}

// ClusterState holds frequently used information about the state of the
// cluster:
// - a map of tainted nodes
// - whether we support disconnected clients
// - current time
type ClusterState struct {
	TaintedNodes                map[string]*structs.Node
	SupportsDisconnectedClients bool
	Now                         time.Time
}

// NewAllocReconciler creates a new reconciler that should be used to determine
// the changes required to bring the cluster state inline with the declared jobspec
func NewAllocReconciler(logger log.Logger, allocUpdateFn AllocUpdateType,
	reconcilerState ReconcilerState, clusterState ClusterState, opts ...AllocReconcilerOption) *AllocReconciler {

	ar := &AllocReconciler{
		logger:             logger.Named("reconciler"),
		allocUpdateFn:      allocUpdateFn,
		jobState:           reconcilerState,
		clusterState:       clusterState,
		reconnectingPicker: newReconnectingPicker(logger),
	}

	for _, op := range opts {
		op(ar)
	}

	return ar
}

// Compute reconciles the existing cluster state and returns the set of changes
// required to converge the job spec and state
func (a *AllocReconciler) Compute() *ReconcileResults {
	result := &ReconcileResults{}

	// Create the allocation matrix
	m := newAllocMatrix(a.jobState.Job, a.jobState.ExistingAllocs)

	a.jobState.DeploymentOld, a.jobState.DeploymentCurrent, result.DeploymentUpdates = cancelUnneededServiceDeployments(a.jobState.Job, a.jobState.DeploymentCurrent)

	// If we are just stopping a job we do not need to do anything more than
	// stopping all running allocs
	if a.jobState.Job.Stopped() {
		desiredTGUpdates, allocsToStop := a.handleStop(m)
		result.DesiredTGUpdates = desiredTGUpdates
		result.Stop = allocsToStop
		return result
	}

	// set deployment paused and failed fields, if we currently have a
	// deployment
	if a.jobState.DeploymentCurrent != nil {
		// deployment is paused when it's manually paused by a user, or if the
		// deployment is pending or initializing, which are the initial states
		// for multi-region job deployments. This flag tells Compute that we
		// should not make placements on the deployment.
		a.jobState.DeploymentPaused = a.jobState.DeploymentCurrent.Status == structs.DeploymentStatusPaused ||
			a.jobState.DeploymentCurrent.Status == structs.DeploymentStatusPending ||
			a.jobState.DeploymentCurrent.Status == structs.DeploymentStatusInitializing
		a.jobState.DeploymentFailed = a.jobState.DeploymentCurrent.Status == structs.DeploymentStatusFailed
	}

	// check if the deployment is complete and set relevant result fields in the
	// process
	var deploymentComplete bool
	result, deploymentComplete = a.computeDeploymentComplete(result, m)

	result.DeploymentUpdates = append(result.DeploymentUpdates, a.setDeploymentStatusAndUpdates(deploymentComplete, result.Deployment)...)

	return result
}

// handleStop marks all allocations to be stopped, handling the lost case.
// Returns result structure with desired changes field set to stopped
// allocations and an array of stopped allocations. It mutates the Stop fields
// on the DesiredUpdates.
func (a *AllocReconciler) handleStop(m allocMatrix) (map[string]*structs.DesiredUpdates, []AllocStopResult) {
	result := make(map[string]*structs.DesiredUpdates)
	allocsToStop := []AllocStopResult{}

	for group, as := range m {
		as = as.filterByTerminal()
		desiredChanges := new(structs.DesiredUpdates)
		desiredChanges.Stop, allocsToStop = as.filterAndStopAll(a.clusterState)
		result[group] = desiredChanges
	}
	return result, allocsToStop
}

// markStop is a helper for marking a set of allocation for stop with a
// particular client status and description. Returns a slice of alloc stop
// result.
func markStop(allocs allocSet, clientStatus, statusDescription string) []AllocStopResult {
	allocsToStop := []AllocStopResult{}
	for _, alloc := range allocs {
		allocsToStop = append(allocsToStop, AllocStopResult{
			Alloc:             alloc,
			ClientStatus:      clientStatus,
			StatusDescription: statusDescription,
		})
	}
	return allocsToStop
}

// markDelayed does markStop, but optionally includes a FollowupEvalID so that we can update
// the stopped alloc with its delayed rescheduling evalID
func markDelayed(allocs allocSet, clientStatus, statusDescription string, followupEvals map[string]string) []AllocStopResult {
	allocsToStop := []AllocStopResult{}
	for _, alloc := range allocs {
		allocsToStop = append(allocsToStop, AllocStopResult{
			Alloc:             alloc,
			ClientStatus:      clientStatus,
			StatusDescription: statusDescription,
			FollowupEvalID:    followupEvals[alloc.ID],
		})
	}
	return allocsToStop
}

// computeDeploymentComplete is the top-level method that computes
// reconciliation for a given allocation matrix. It returns ReconcileResults
// struct and a boolean that indicates whether the deployment is complete.
func (a *AllocReconciler) computeDeploymentComplete(result *ReconcileResults, m allocMatrix) (*ReconcileResults, bool) {
	complete := true
	for group, as := range m {
		var groupComplete bool
		var resultForGroup *ReconcileResults
		resultForGroup, groupComplete = a.computeGroup(group, as)
		complete = complete && groupComplete

		// merge results for group with overall results
		result.Merge(resultForGroup)
	}

	return result, complete
}

// computeGroup reconciles state for a particular task group. It returns whether
// the deployment it is for is complete in regard to the task group.
//
// returns: ReconcileResults object and a boolean that indicates whether the
// whole group's deployment is complete
func (a *AllocReconciler) computeGroup(group string, all allocSet) (*ReconcileResults, bool) {

	// Create the output result object that we'll be continuously writing to
	result := new(ReconcileResults)
	result.DesiredTGUpdates = make(map[string]*structs.DesiredUpdates)
	result.DesiredTGUpdates[group] = new(structs.DesiredUpdates)

	// Get the task group. The task group may be nil if the job was updates such
	// that the task group no longer exists
	tg := a.jobState.Job.LookupTaskGroup(group)

	all = all.filterServerTerminalAllocs()

	// If the task group is nil or scaled-to-zero, then the task group has been
	// removed so all we need to do is stop everything
	if tg == nil || tg.Count == 0 {
		result.DesiredTGUpdates[group].Stop, result.Stop = all.filterAndStopAll(a.clusterState)
		return result, true
	}

	dstate, existingDeployment := a.initializeDeploymentState(group, tg)

	// Filter allocations that do not need to be considered because they are
	// from an older job version and are terminal.
	all, ignore := all.filterOldTerminalAllocs(a.jobState)
	result.DesiredTGUpdates[group].Ignore += uint64(len(ignore))

	canaries := a.cancelUnneededCanaries(&all, group, result)

	// Determine what set of allocations are on tainted nodes
	untainted, migrate, lost, disconnecting, reconnecting, ignore, expiring := all.filterByTainted(a.clusterState)
	result.DesiredTGUpdates[group].Ignore += uint64(len(ignore))

	// Determine what set of terminal allocations need to be rescheduled
	untainted, rescheduleNow, rescheduleLater := untainted.filterByRescheduleable(
		a.jobState.JobIsBatch, false, a.clusterState.Now,
		a.jobState.EvalID, a.jobState.DeploymentCurrent)

	// Determine what set of migrating allocations need to be rescheduled. These
	// will be batch job allocations that were stopped using the `stop alloc` command.
	_, migrateRescheduleNow, migrateRescheduleLater := migrate.filterByRescheduleable(
		a.jobState.JobIsBatch, false, a.clusterState.Now,
		a.jobState.EvalID, a.jobState.DeploymentCurrent)

	rescheduleNow = rescheduleNow.union(migrateRescheduleNow)
	rescheduleLater = append(rescheduleLater, migrateRescheduleLater...)

	// If there are allocations reconnecting we need to reconcile them and their
	// replacements first because there is specific logic when deciding which
	// ones to keep that can only be applied when the client reconnects.
	if len(reconnecting) > 0 {
		a.computeReconnecting(&untainted, &migrate, &lost, &disconnecting,
			reconnecting, all, tg, result)
	}

	if len(expiring) > 0 {
		if !tg.Replace() {
			untainted = untainted.union(expiring)
		} else {
			lost = lost.union(expiring)
		}
	}

	result.DesiredFollowupEvals = map[string][]*structs.Evaluation{}
	result.DisconnectUpdates = make(allocSet)

	// Determine what set of disconnecting allocations need to be rescheduled
	// now, which ones later and which ones can't be rescheduled at all.
	timeoutLaterEvals := map[string]string{}
	if len(disconnecting) > 0 {
		timeoutLaterEvals = a.computeDisconnecting(
			disconnecting,
			&untainted,
			&rescheduleNow,
			&rescheduleLater,
			tg,
			result,
		)
	}

	// Find delays for any lost allocs that have disconnect.stop_on_client_after
	lostLaterEvals := map[string]string{}
	lostLater := []*delayedRescheduleInfo{} // guards computePlacements

	if len(lost) > 0 {
		lostLater = lost.delayByStopAfter()
		var followupEvals []*structs.Evaluation
		lostLaterEvals, followupEvals = a.createLaterEvals(lostLater, structs.EvalTriggerRetryFailedAlloc)
		result.DesiredFollowupEvals[tg.Name] = append(result.DesiredFollowupEvals[tg.Name], followupEvals...)
	}

	// Merge evals for disconnecting with the disconnect.stop_on_client_after
	// set into the lostLaterEvals so that computeStop can add them to the stop
	// set.
	maps.Copy(lostLaterEvals, timeoutLaterEvals)

	if len(rescheduleLater) > 0 {
		// Create batched follow-up evaluations for allocations that are
		// reschedulable later and mark the allocations for in place updating
		a.createRescheduleLaterEvals(rescheduleLater, all, migrate, tg.Name, result)
	}

	// Create a structure for choosing names. Seed with the taken names
	// which is the union of untainted, rescheduled, allocs on migrating
	// nodes, and allocs on down nodes (includes canaries)
	nameIndex := newAllocNameIndex(a.jobState.JobID, group, tg.Count, untainted.union(migrate, rescheduleNow, lost))
	allocNameIndexForGroup := nameIndex
	result.TaskGroupAllocNameIndexes = map[string]*AllocNameIndex{group: allocNameIndexForGroup}

	// Stop any unneeded allocations and update the untainted set to not include
	// stopped allocations.
	isCanarying := dstate != nil && dstate.DesiredCanaries != 0 && !dstate.Promoted
	a.computeStop(tg, nameIndex, &untainted, migrate, lost, canaries,
		isCanarying, lostLaterEvals, result)

	// Do inplace upgrades where possible and capture the set of upgrades that
	// need to be done destructively.
	inplace, destructive := a.computeUpdates(untainted, tg, result)
	if !existingDeployment {
		dstate.DesiredTotal += len(destructive) + len(inplace)
	}

	// Remove the canaries now that we have handled rescheduling so that we do
	// not consider them when making placement decisions.
	if isCanarying {
		untainted = untainted.difference(canaries)
	}
	requiresCanaries := requiresCanaries(tg, dstate, destructive, canaries)
	if requiresCanaries {
		a.computeCanaries(tg, dstate, destructive, canaries, nameIndex, group, result)
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
	var place []AllocPlaceResult
	if len(lostLater) == 0 {
		place = computePlacements(tg, nameIndex, untainted, migrate, rescheduleNow, lost, disconnecting, isCanarying)
		if !existingDeployment {
			dstate.DesiredTotal += len(place)
		}
	}

	// deploymentPlaceReady tracks whether the deployment is in a state where
	// placements can be made without any other consideration.
	deploymentPlaceReady := !a.jobState.DeploymentPaused && !a.jobState.DeploymentFailed && !isCanarying

	underProvisionedBy, replacements, replacementsAllocsToStop := a.placeAllocs(
		deploymentPlaceReady, result.DesiredTGUpdates[group], place, migrate, rescheduleNow, lost, result.DisconnectUpdates, underProvisionedBy)
	result.Stop = append(result.Stop, replacementsAllocsToStop...)
	result.Place = append(result.Place, replacements...)

	if deploymentPlaceReady {
		result.DestructiveUpdate = a.computeDestructiveUpdates(destructive, underProvisionedBy, result.DesiredTGUpdates[group], tg)
	} else {
		result.DesiredTGUpdates[group].Ignore += uint64(len(destructive))
	}

	a.computeMigrations(migrate, isCanarying, tg, result)
	result.Deployment = a.createDeployment(
		tg.Name, tg.Update, existingDeployment, dstate, all, destructive, int(result.DesiredTGUpdates[group].InPlaceUpdate))

	// Deployments that are still initializing need to be sent in full in the
	// plan so its internal state can be persisted by the plan applier.
	if a.jobState.DeploymentCurrent != nil && a.jobState.DeploymentCurrent.Status == structs.DeploymentStatusInitializing {
		result.Deployment = a.jobState.DeploymentCurrent
	}

	// We can never have more placements than the count
	if len(result.Place) > tg.Count {
		result.Place = result.Place[:tg.Count]
		result.DesiredTGUpdates[tg.Name].Place = uint64(tg.Count)
	}

	deploymentComplete := a.isDeploymentComplete(group, destructive, inplace,
		migrate, rescheduleNow, result.Place, rescheduleLater, requiresCanaries)

	return result, deploymentComplete
}

// cancelUnneededServiceDeployments cancels any deployment that is not needed.
// A deployment update will be staged for jobs that should stop or have the
// wrong version. Unneeded deployments include:
// 1. Jobs that are marked for stop, but there is a non-terminal deployment.
// 2. Deployments that are active, but referencing a different job version.
// 3. Deployments that are already successful.
//
// returns: old deployment, current deployment and a slice of deployment status
// updates.
func cancelUnneededServiceDeployments(j *structs.Job, d *structs.Deployment) (*structs.Deployment, *structs.Deployment, []*structs.DeploymentStatusUpdate) {
	var updates []*structs.DeploymentStatusUpdate

	// If the job is stopped and there is a non-terminal deployment, cancel it
	if j.Stopped() {
		if d != nil && d.Active() {
			updates = append(updates, &structs.DeploymentStatusUpdate{
				DeploymentID:      d.ID,
				Status:            structs.DeploymentStatusCancelled,
				StatusDescription: structs.DeploymentStatusDescriptionStoppedJob,
			})
		}

		// Nothing else to do
		return d, nil, updates
	}

	if d == nil {
		return nil, nil, nil
	}

	// Check if the deployment is active and referencing an older job and cancel it
	if d.JobCreateIndex != j.CreateIndex || d.JobVersion != j.Version {
		if d.Active() {
			updates = append(updates, &structs.DeploymentStatusUpdate{
				DeploymentID:      d.ID,
				Status:            structs.DeploymentStatusCancelled,
				StatusDescription: structs.DeploymentStatusDescriptionNewerJob,
			})
		}

		return d, nil, updates
	}

	// Clear it as the current deployment if it is successful
	if d.Status == structs.DeploymentStatusSuccessful {
		return d, nil, updates
	}

	return nil, d, updates
}

// setDeploymentStatusAndUpdates sets status for a.deployment if necessary and
// returns an array of DeploymentStatusUpdates.
func (a *AllocReconciler) setDeploymentStatusAndUpdates(deploymentComplete bool, createdDeployment *structs.Deployment) []*structs.DeploymentStatusUpdate {
	var updates []*structs.DeploymentStatusUpdate

	if a.jobState.DeploymentCurrent != nil {
		// Mark the deployment as complete if possible
		if deploymentComplete {
			if a.jobState.Job.IsMultiregion() {
				// the unblocking/successful states come after blocked, so we
				// need to make sure we don't revert those states
				if a.jobState.DeploymentCurrent.Status != structs.DeploymentStatusUnblocking &&
					a.jobState.DeploymentCurrent.Status != structs.DeploymentStatusSuccessful {
					updates = append(updates, &structs.DeploymentStatusUpdate{
						DeploymentID:      a.jobState.DeploymentCurrent.ID,
						Status:            structs.DeploymentStatusBlocked,
						StatusDescription: structs.DeploymentStatusDescriptionBlocked,
					})
				}
			} else {
				updates = append(updates, &structs.DeploymentStatusUpdate{
					DeploymentID:      a.jobState.DeploymentCurrent.ID,
					Status:            structs.DeploymentStatusSuccessful,
					StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
				})
			}
		}

		// Mark the deployment as pending since its state is now computed.
		if a.jobState.DeploymentCurrent.Status == structs.DeploymentStatusInitializing {
			updates = append(updates, &structs.DeploymentStatusUpdate{
				DeploymentID:      a.jobState.DeploymentCurrent.ID,
				Status:            structs.DeploymentStatusPending,
				StatusDescription: structs.DeploymentStatusDescriptionPendingForPeer,
			})
		}
	}

	// Set the description of a created deployment
	if createdDeployment != nil {
		if createdDeployment.RequiresPromotion() {
			if createdDeployment.HasAutoPromote() {
				createdDeployment.StatusDescription = structs.DeploymentStatusDescriptionRunningAutoPromotion
			} else {
				createdDeployment.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
			}
		}
	}
	return updates
}

func (a *AllocReconciler) initializeDeploymentState(group string, tg *structs.TaskGroup) (*structs.DeploymentState, bool) {
	var dstate *structs.DeploymentState
	existingDeployment := false

	if a.jobState.DeploymentCurrent != nil {
		dstate, existingDeployment = a.jobState.DeploymentCurrent.TaskGroups[group]
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
func requiresCanaries(tg *structs.TaskGroup, dstate *structs.DeploymentState, destructive, canaries allocSet) bool {
	canariesPromoted := dstate != nil && dstate.Promoted
	return tg.Update != nil &&
		len(destructive) != 0 &&
		len(canaries) < tg.Update.Canary &&
		!canariesPromoted
}

// computeCanaries returns the set of new canaries to place. It mutates the
// Canary field on the DesiredUpdates and the DesiredCanaries on the dstate
func (a *AllocReconciler) computeCanaries(
	tg *structs.TaskGroup, dstate *structs.DeploymentState,
	destructive, canaries allocSet,
	nameIndex *AllocNameIndex,
	group string,
	result *ReconcileResults,
) {

	dstate.DesiredCanaries = tg.Update.Canary

	placementResult := []AllocPlaceResult{}

	if !a.jobState.DeploymentPaused && !a.jobState.DeploymentFailed {
		result.DesiredTGUpdates[group].Canary += uint64(tg.Update.Canary - len(canaries))
		total := uint(result.DesiredTGUpdates[group].Canary)

		for _, name := range nameIndex.NextCanaries(total, canaries, destructive) {
			placementResult = append(placementResult, AllocPlaceResult{
				name:      name,
				canary:    true,
				taskGroup: tg,
			})
		}
	}

	result.Place = append(result.Place, placementResult...)
}

// cancelUnneededCanaries handles the canaries for the group by stopping the
// unneeded ones and returning the current set of canaries and the updated total
// set of allocs for the group
func (a *AllocReconciler) cancelUnneededCanaries(all *allocSet, group string, result *ReconcileResults) (
	canaries allocSet) {

	// Stop any canary from an older deployment or from a failed one
	var stop []string

	// Cancel any non-promoted canaries from the older deployment
	if a.jobState.DeploymentOld != nil {
		for _, dstate := range a.jobState.DeploymentOld.TaskGroups {
			if !dstate.Promoted {
				stop = append(stop, dstate.PlacedCanaries...)
			}
		}
	}

	// Cancel any non-promoted canaries from a failed deployment
	if a.jobState.DeploymentCurrent != nil && a.jobState.DeploymentCurrent.Status == structs.DeploymentStatusFailed {
		for _, dstate := range a.jobState.DeploymentCurrent.TaskGroups {
			if !dstate.Promoted {
				stop = append(stop, dstate.PlacedCanaries...)
			}
		}
	}

	// stopSet is the allocSet that contains the canaries we desire to stop from
	// above.
	stopSet := all.fromKeys(stop)
	allocsToStop := markStop(stopSet, "", sstructs.StatusAllocNotNeeded)
	result.DesiredTGUpdates[group].Stop += uint64(len(stopSet))
	*all = all.difference(stopSet)

	// Capture our current set of canaries and handle any migrations that are
	// needed by just stopping them.
	if a.jobState.DeploymentCurrent != nil {
		var canaryIDs []string
		for _, dstate := range a.jobState.DeploymentCurrent.TaskGroups {
			canaryIDs = append(canaryIDs, dstate.PlacedCanaries...)
		}

		canaries = all.fromKeys(canaryIDs)
		untainted, migrate, lost, _, _, _, _ := canaries.filterByTainted(a.clusterState)

		// We don't add these stops to desiredChanges because the deployment is
		// still active. DesiredChanges is used to report deployment progress/final
		// state. These transient failures aren't meaningful.
		allocsToStop = slices.Concat(allocsToStop,
			markStop(migrate, "", sstructs.StatusAllocMigrating),
			markStop(lost, structs.AllocClientStatusLost, sstructs.StatusAllocLost),
		)

		canaries = untainted
		*all = all.difference(migrate, lost)
	}

	result.Stop = allocsToStop
	return canaries
}

// computeUnderProvisionedBy returns the number of allocs that still need to be
// placed for a particular group. The inputs are the group definition, the untainted,
// destructive, and migrate allocation sets, and whether we are in a canary state.
func (a *AllocReconciler) computeUnderProvisionedBy(group *structs.TaskGroup, untainted, destructive, migrate allocSet, isCanarying bool) int {
	// If no update strategy, nothing is migrating, and nothing is being replaced,
	// allow as many as defined in group.Count
	if group.Update.IsEmpty() || len(destructive)+len(migrate) == 0 {
		return group.Count
	}

	// If the deployment is nil, allow MaxParallel placements
	if a.jobState.DeploymentCurrent == nil {
		return group.Update.MaxParallel
	}

	// If the deployment is paused, failed, or we have un-promoted canaries, do not create anything else.
	if a.jobState.DeploymentPaused ||
		a.jobState.DeploymentFailed ||
		isCanarying {
		return 0
	}

	underProvisionedBy := group.Update.MaxParallel
	partOf, _ := untainted.filterByDeployment(a.jobState.DeploymentCurrent.ID)
	for _, alloc := range partOf {
		// An unhealthy allocation means nothing else should happen.
		if alloc.DeploymentStatus.IsUnhealthy() {
			return 0
		}
		// If not yet explicitly set to healthy (nil) decrement.
		if !alloc.DeploymentStatus.IsHealthy() {
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
func computePlacements(group *structs.TaskGroup,
	nameIndex *AllocNameIndex, untainted, migrate, reschedule, lost, disconnected allocSet,
	isCanarying bool) []AllocPlaceResult {

	// Add rescheduled placement results
	var place []AllocPlaceResult
	for _, alloc := range reschedule {
		place = append(place, AllocPlaceResult{
			name:          alloc.Name,
			taskGroup:     group,
			previousAlloc: alloc,
			reschedule:    true,
			canary:        alloc.DeploymentStatus.IsCanary(),

			downgradeNonCanary: isCanarying && !alloc.DeploymentStatus.IsCanary(),
			minJobVersion:      alloc.Job.Version,
			lost:               false,
		})
	}

	// Add replacements for lost allocs up to group.Count
	existing := len(untainted) + len(migrate) + len(reschedule) + len(disconnected)

	// Add replacements for lost
	for _, alloc := range lost {
		if existing >= group.Count {
			// Reached desired count, do not replace remaining lost
			// allocs
			break
		}

		existing++
		place = append(place, AllocPlaceResult{
			name:               alloc.Name,
			taskGroup:          group,
			previousAlloc:      alloc,
			reschedule:         false,
			canary:             alloc.DeploymentStatus.IsCanary(),
			downgradeNonCanary: isCanarying && !alloc.DeploymentStatus.IsCanary(),
			minJobVersion:      alloc.Job.Version,
			lost:               true,
		})
	}

	// Add remaining placement results
	if existing < group.Count {
		for _, name := range nameIndex.Next(uint(group.Count - existing)) {
			place = append(place, AllocPlaceResult{
				name:               name,
				taskGroup:          group,
				downgradeNonCanary: isCanarying,
			})
		}
	}

	return place
}

// placeAllocs either applies the placements calculated by computePlacements,
// or computes more placements based on whether the deployment is ready for
// and if allocations are already rescheduling or part of a failed
// deployment. The input deploymentPlaceReady is calculated as the deployment
// is not paused, failed, or canarying. It returns the number of allocs still
// needed, allocations to place, and allocations to stop.
func (a *AllocReconciler) placeAllocs(deploymentPlaceReady bool, desiredChanges *structs.DesiredUpdates,
	place []AllocPlaceResult, migrate, rescheduleNow, lost allocSet, disconnectUpdates allocSet,
	underProvisionedBy int) (int, []AllocPlaceResult, []AllocStopResult) {

	// Disconnecting and migrating allocs are not failing, but may be included
	// in rescheduleNow. Create a new set that only includes the actual failures
	// and compute replacements based off that.
	failed := make(allocSet)
	for id, alloc := range rescheduleNow {
		_, isDisconnecting := disconnectUpdates[id]
		_, isMigrating := migrate[id]
		if !isDisconnecting && !isMigrating && alloc.ClientStatus != structs.AllocClientStatusUnknown {
			failed[id] = alloc
		}
	}

	resultingPlacements := []AllocPlaceResult{}
	resultingAllocsToStop := []AllocStopResult{}

	// If the deployment is place ready, apply all placements and return
	if deploymentPlaceReady {
		desiredChanges.Place += uint64(len(place))
		// This relies on the computePlacements having built this set, which in
		// turn relies on len(lostLater) == 0.
		resultingPlacements = append(resultingPlacements, place...)

		resultingAllocsToStop = markStop(failed, "", sstructs.StatusAllocRescheduled)
		desiredChanges.Stop += uint64(len(failed))

		minimum := min(len(place), underProvisionedBy)
		underProvisionedBy -= minimum
		return underProvisionedBy, resultingPlacements, resultingAllocsToStop
	}

	// We do not want to place additional allocations but in the case we
	// have lost allocations or allocations that require rescheduling now,
	// we do so regardless to avoid odd user experiences.

	// If allocs have been lost, determine the number of replacements that are needed
	// and add placements to the result for the lost allocs.
	if len(lost) != 0 {
		allowed := min(len(lost), len(place))
		desiredChanges.Place += uint64(allowed)
		resultingPlacements = append(resultingPlacements, place[:allowed]...)
	}

	// if no failures or there are no pending placements return.
	if len(rescheduleNow) == 0 || len(place) == 0 {
		return underProvisionedBy, resultingPlacements, nil
	}

	// Handle rescheduling of failed allocations even if the deployment is failed.
	// If the placement is rescheduling, and not part of a failed deployment, add
	// to the place set. Add the previous alloc to the stop set unless it is disconnecting.
	for _, p := range place {
		prev := p.PreviousAllocation()
		partOfFailedDeployment := a.jobState.DeploymentFailed && prev != nil && a.jobState.DeploymentCurrent.ID == prev.DeploymentID

		if !partOfFailedDeployment && p.IsRescheduling() {
			resultingPlacements = append(resultingPlacements, p)
			desiredChanges.Place++

			_, prevIsDisconnecting := disconnectUpdates[prev.ID]
			if prevIsDisconnecting {
				continue
			}

			resultingAllocsToStop = append(resultingAllocsToStop, AllocStopResult{
				Alloc:             prev,
				StatusDescription: sstructs.StatusAllocRescheduled,
			})
			desiredChanges.Stop++
		}
	}

	return underProvisionedBy, resultingPlacements, resultingAllocsToStop
}

// computeDestructiveUpdates returns the set of destructive updates. It mutates
// the DestructiveUpdate and Ignore fields on the DesiredUpdates counts
func (a *AllocReconciler) computeDestructiveUpdates(destructive allocSet, underProvisionedBy int,
	desiredChanges *structs.DesiredUpdates, tg *structs.TaskGroup) []allocDestructiveResult {

	destructiveResult := []allocDestructiveResult{}

	// Do all destructive updates
	minimum := min(len(destructive), underProvisionedBy)
	desiredChanges.DestructiveUpdate += uint64(minimum)
	desiredChanges.Ignore += uint64(len(destructive) - minimum)
	for _, alloc := range destructive.nameOrder()[:minimum] {
		destructiveResult = append(destructiveResult, allocDestructiveResult{
			placeName:             alloc.Name,
			placeTaskGroup:        tg,
			stopAlloc:             alloc,
			stopStatusDescription: sstructs.StatusAllocUpdating,
		})
	}

	return destructiveResult
}

// computeMigrations updates the result with the stops and placements required
// for migration.
func (a *AllocReconciler) computeMigrations(migrate allocSet, isCanarying bool,
	tg *structs.TaskGroup, result *ReconcileResults) {

	result.DesiredTGUpdates[tg.Name].Migrate += uint64(len(migrate))

	for _, alloc := range migrate.nameOrder() {
		result.Stop = append(result.Stop, AllocStopResult{
			Alloc:             alloc,
			StatusDescription: sstructs.StatusAllocMigrating,
		})

		// If this is a batch job allocation, check if the allocation should
		// be placed. If the allocation should be rescheduled, the reschedule
		// logic will handle placement and it should not be done here (used
		// by the `alloc stop` command). If the allocation should disable
		// migration placement, then placment should not be done here (used
		// when draining batch allocations).
		if alloc.Job.Type == structs.JobTypeBatch && (alloc.DesiredTransition.ShouldReschedule() || alloc.DesiredTransition.ShouldDisableMigrationPlacement()) {
			continue
		}

		result.Place = append(result.Place, AllocPlaceResult{
			name:          alloc.Name,
			canary:        alloc.DeploymentStatus.IsCanary(),
			taskGroup:     tg,
			previousAlloc: alloc,

			downgradeNonCanary: isCanarying && !alloc.DeploymentStatus.IsCanary(),
			minJobVersion:      alloc.Job.Version,
		})
	}
}

// createDeployment creates a new deployment if necessary.
// WARNING: this method mutates reconciler state field deploymentCurrent
func (a *AllocReconciler) createDeployment(groupName string, strategy *structs.UpdateStrategy,
	existingDeployment bool, dstate *structs.DeploymentState, all, destructive allocSet, inPlaceUpdates int) *structs.Deployment {
	// Guard the simple cases that require no computation first.
	if existingDeployment ||
		strategy.IsEmpty() ||
		dstate.DesiredTotal == 0 {
		return nil
	}

	updatingSpec := len(destructive) != 0 || inPlaceUpdates != 0

	hadRunning := false
	for _, alloc := range all {
		if alloc.Job.Version == a.jobState.Job.Version && alloc.Job.CreateIndex == a.jobState.Job.CreateIndex {
			hadRunning = true
			break
		}
	}

	// Don't create a deployment if it's not the first time running the job
	// and there are no updates to the spec.
	if hadRunning && !updatingSpec {
		return nil
	}

	var resultingDeployment *structs.Deployment

	// A previous group may have made the deployment already. If not create one.
	if a.jobState.DeploymentCurrent == nil {
		a.jobState.DeploymentCurrent = structs.NewDeployment(a.jobState.Job, a.jobState.EvalPriority, a.clusterState.Now.UnixNano())
		resultingDeployment = a.jobState.DeploymentCurrent
	}

	// Attach the groups deployment state to the deployment
	a.jobState.DeploymentCurrent.TaskGroups[groupName] = dstate

	return resultingDeployment
}

func (a *AllocReconciler) isDeploymentComplete(groupName string, destructive, inplace, migrate, rescheduleNow allocSet,
	place []AllocPlaceResult, rescheduleLater []*delayedRescheduleInfo, requiresCanaries bool) bool {

	complete := len(destructive)+len(inplace)+len(place)+len(migrate)+len(rescheduleNow)+len(rescheduleLater) == 0 &&
		!requiresCanaries

	if !complete || a.jobState.DeploymentCurrent == nil {
		return false
	}

	// Final check to see if the deployment is complete is to ensure everything is healthy
	if dstate, ok := a.jobState.DeploymentCurrent.TaskGroups[groupName]; ok {
		if dstate.HealthyAllocs < max(dstate.DesiredTotal, dstate.DesiredCanaries) || // Make sure we have enough healthy allocs
			(dstate.DesiredCanaries > 0 && !dstate.Promoted) { // Make sure we are promoted if we have canaries
			complete = false
		}
	}

	return complete
}

// computeStop updates the result with the set of allocations we want to stop
// given the group definition, the set of allocations in various states and
// whether we are canarying. It mutates the untainted set with the remaining
// allocations.
func (a *AllocReconciler) computeStop(group *structs.TaskGroup, nameIndex *AllocNameIndex,
	untainted *allocSet, migrate, lost, canaries allocSet,
	isCanarying bool, followupEvals map[string]string, result *ReconcileResults) {

	// Mark all lost allocations for stop and copy the original untainted set as
	// our working set (so that we only mutate the untainted set at the end)
	var stop, working allocSet
	stop = stop.union(lost)
	working = working.union(*untainted)

	var stopAllocResult []AllocStopResult

	defer func() {
		result.Stop = append(result.Stop, stopAllocResult...)
		result.DesiredTGUpdates[group.Name].Stop += uint64(len(stop))
		*untainted = untainted.difference(stop)
	}()

	delayedResult := markDelayed(lost, structs.AllocClientStatusLost, sstructs.StatusAllocLost, followupEvals)
	stopAllocResult = append(stopAllocResult, delayedResult...)

	// If we are still deploying or creating canaries, don't stop them
	if isCanarying {
		working = working.difference(canaries)
	}

	// Remove disconnected allocations so they won't be stopped
	knownUntainted := working.filterOutByClientStatus(structs.AllocClientStatusUnknown)

	// Hot path the nothing to do case
	//
	// Note that this path can result in duplicated allocation indexes in a
	// scenario where a destructive job change (ex. image update) happens with
	// an increased group count. Once the canary is replaced, and we compute
	// the next set of stops, the untainted set equals the new group count,
	// which results is missing one removal. The duplicate alloc index is
	// corrected in `computePlacements`
	remove := len(knownUntainted) + len(migrate) - group.Count
	if remove <= 0 {
		return
	}

	// Filter out any terminal allocations from the untainted set
	// This is so that we don't try to mark them as stopped redundantly
	working = working.filterByTerminal()

	// Prefer stopping any alloc that has the same name as the canaries if we
	// are promoted
	if !isCanarying && len(canaries) != 0 {
		canaryNames := canaries.nameSet()
		for id, alloc := range working.difference(canaries) {
			if _, match := canaryNames[alloc.Name]; match {
				stop[id] = alloc
				stopAllocResult = append(stopAllocResult, AllocStopResult{
					Alloc:             alloc,
					StatusDescription: sstructs.StatusAllocNotNeeded,
				})
				delete(working, id)

				remove--
				if remove == 0 {
					return
				}
			}
		}
	}

	// Prefer selecting from the migrating set before stopping existing allocs
	if len(migrate) != 0 {
		migratingNames := newAllocNameIndex(a.jobState.JobID, group.Name, group.Count, migrate)
		removeNames := migratingNames.Highest(uint(remove))
		for id, alloc := range migrate {
			if _, match := removeNames[alloc.Name]; !match {
				continue
			}
			stopAllocResult = append(stopAllocResult, AllocStopResult{
				Alloc:             alloc,
				StatusDescription: sstructs.StatusAllocNotNeeded,
			})
			delete(migrate, id)
			stop[id] = alloc
			nameIndex.UnsetIndex(alloc.Index())

			remove--
			if remove == 0 {
				return
			}
		}
	}

	// Select the allocs with the highest count to remove
	removeNames := nameIndex.Highest(uint(remove))
	for id, alloc := range working {
		if _, ok := removeNames[alloc.Name]; ok {
			stop[id] = alloc
			stopAllocResult = append(stopAllocResult, AllocStopResult{
				Alloc:             alloc,
				StatusDescription: sstructs.StatusAllocNotNeeded,
			})
			delete(working, id)

			remove--
			if remove == 0 {
				return
			}
		}
	}

	// It is possible that we didn't stop as many as we should have if there
	// were allocations with duplicate names.
	for id, alloc := range working {
		stop[id] = alloc
		stopAllocResult = append(stopAllocResult, AllocStopResult{
			Alloc:             alloc,
			StatusDescription: sstructs.StatusAllocNotNeeded,
		})
		delete(working, id)

		remove--
		if remove == 0 {
			return
		}
	}
}

// If there are allocations reconnecting we need to reconcile them and their
// replacements first because there is specific logic when deciding which ones
// to keep that can only be applied when the client reconnects.
func (a *AllocReconciler) computeReconnecting(
	untainted, migrate, lost, disconnecting *allocSet, reconnecting, all allocSet,
	tg *structs.TaskGroup, result *ReconcileResults) {

	// Pass all allocations because the replacements we need to find may be in
	// any state, including themselves being reconnected.
	reconnect, stopAllocSet, stopAllocResult := a.reconcileReconnecting(reconnecting, all, tg)
	result.Stop = append(result.Stop, stopAllocResult...)

	// Stop the reconciled allocations and remove them from untainted, migrate,
	// lost and disconnecting sets, since they have been already handled.
	result.DesiredTGUpdates[tg.Name].Stop += uint64(len(stopAllocSet))

	*untainted = untainted.difference(stopAllocSet)
	*migrate = migrate.difference(stopAllocSet)
	*lost = lost.difference(stopAllocSet)
	*disconnecting = disconnecting.difference(stopAllocSet)

	// Validate and add reconnecting allocations to the plan so they are
	// logged.
	if len(reconnect) > 0 {
		result.ReconnectUpdates = a.appendReconnectingUpdates(reconnect)
		result.DesiredTGUpdates[tg.Name].Reconnect = uint64(len(result.ReconnectUpdates))

		// The rest of the reconnecting allocations are now untainted and will
		// be further reconciled below.
		*untainted = untainted.union(reconnect)
	}
}

// reconcileReconnecting receives the set of allocations that are reconnecting
// and all other allocations for the same group and determines which ones to
// reconnect, which ones to stop, and the stop results for the latter.
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
func (a *AllocReconciler) reconcileReconnecting(reconnecting allocSet, all allocSet, tg *structs.TaskGroup) (allocSet, allocSet, []AllocStopResult) {
	stop := make(allocSet)
	reconnect := make(allocSet)
	stopAllocResult := []AllocStopResult{}

	for _, reconnectingAlloc := range reconnecting {

		// Stop allocations that failed to reconnect.
		reconnectFailed := !reconnectingAlloc.ServerTerminalStatus() &&
			reconnectingAlloc.ClientStatus == structs.AllocClientStatusFailed

		if reconnectFailed {
			stop[reconnectingAlloc.ID] = reconnectingAlloc
			stopAllocResult = append(stopAllocResult, AllocStopResult{
				Alloc:             reconnectingAlloc,
				ClientStatus:      structs.AllocClientStatusFailed,
				StatusDescription: sstructs.StatusAllocRescheduled,
			})
			continue
		}

		// If the desired status is not run, or if the user-specified desired
		// transition is not run, stop the reconnecting allocation.
		stopReconnecting := reconnectingAlloc.DesiredStatus != structs.AllocDesiredStatusRun ||
			reconnectingAlloc.DesiredTransition.ShouldMigrate() ||
			reconnectingAlloc.DesiredTransition.ShouldReschedule() ||
			reconnectingAlloc.DesiredTransition.ShouldForceReschedule() ||
			reconnectingAlloc.Job.Version < a.jobState.Job.Version ||
			reconnectingAlloc.Job.CreateIndex < a.jobState.Job.CreateIndex

		if stopReconnecting {
			stop[reconnectingAlloc.ID] = reconnectingAlloc
			stopAllocResult = append(stopAllocResult, AllocStopResult{
				Alloc:             reconnectingAlloc,
				StatusDescription: sstructs.StatusAllocNotNeeded,
			})
			continue
		}

		// A replacement allocation could fail and be replaced with another
		// so follow the replacements in a linked list style
		replacements := []string{}
		nextAlloc := reconnectingAlloc.NextAllocation
		for {
			val, ok := all[nextAlloc]
			if !ok {
				break
			}
			replacements = append(replacements, val.ID)
			nextAlloc = val.NextAllocation
		}

		// Find replacement allocations and decide which one to stop. A
		// reconnecting allocation may have multiple replacements.
		for _, replacementAlloc := range all {

			// Skip the allocation if it is the reconnecting alloc
			if replacementAlloc == reconnectingAlloc {
				continue
			}

			// Skip allocations that are server terminal or not replacements.
			// We don't want to replace a reconnecting allocation with one that
			// is or will terminate and we don't need to stop them since they
			// are already marked as terminal by the servers.
			if !slices.Contains(replacements, replacementAlloc.ID) || replacementAlloc.ServerTerminalStatus() {
				continue
			}

			// Pick which allocation we want to keep using the disconnect reconcile strategy
			keepAlloc := a.reconnectingPicker.pickReconnectingAlloc(tg.Disconnect, reconnectingAlloc, replacementAlloc)
			if keepAlloc == replacementAlloc {
				// The replacement allocation is preferred, so stop the one
				// reconnecting if not stopped yet.
				if _, ok := stop[reconnectingAlloc.ID]; !ok {
					stop[reconnectingAlloc.ID] = reconnectingAlloc
					stopAllocResult = append(stopAllocResult, AllocStopResult{
						Alloc:             reconnectingAlloc,
						StatusDescription: sstructs.StatusAllocNotNeeded,
					})
				}
			} else {
				// The reconnecting allocation is preferred, so stop any replacements
				// that are not in server terminal status or stopped already.
				if _, ok := stop[replacementAlloc.ID]; !ok {
					stop[replacementAlloc.ID] = replacementAlloc
					stopAllocResult = append(stopAllocResult, AllocStopResult{
						Alloc:             replacementAlloc,
						StatusDescription: sstructs.StatusAllocReconnected,
					})
				}
			}
		}
	}

	// Any reconnecting allocation not set to stop must be reconnected.
	for _, alloc := range reconnecting {
		if _, ok := stop[alloc.ID]; !ok {
			reconnect[alloc.ID] = alloc
		}
	}

	return reconnect, stop, stopAllocResult
}

// computeUpdates determines which allocations for the passed group require
// updates. This method updates the results with allocs to ignore and/or
// update. And two groups are returned:
// 1. Those that can be upgraded in-place
// 2. Those that require destructive updates
func (a *AllocReconciler) computeUpdates(
	untainted allocSet, group *structs.TaskGroup, result *ReconcileResults,
) (inplace, destructive allocSet) {

	ignore := make(allocSet)
	inplace = make(allocSet)
	destructive = make(allocSet)

	for _, alloc := range untainted {
		ignoreChange, destructiveChange, inplaceAlloc := a.allocUpdateFn(alloc, a.jobState.Job, group)
		if ignoreChange {
			ignore[alloc.ID] = alloc
		} else if destructiveChange {
			destructive[alloc.ID] = alloc
		} else {
			inplace[alloc.ID] = inplaceAlloc
		}
	}

	result.InplaceUpdate = slices.Collect(maps.Values(inplace))
	result.DesiredTGUpdates[group.Name].Ignore += uint64(len(ignore))
	result.DesiredTGUpdates[group.Name].InPlaceUpdate += uint64(len(inplace))

	return
}

// createRescheduleLaterEvals creates batched followup evaluations with the
// WaitUntil field set for allocations that are eligible to be rescheduled
// later, and marks the alloc with the FollowupEvalID in the result.
// TODO(tgross): this needs a better name?
func (a *AllocReconciler) createRescheduleLaterEvals(
	rescheduleLater []*delayedRescheduleInfo,
	all allocSet,
	migrate allocSet,
	group string,
	result *ReconcileResults) {

	allocIDToFollowupEvalID, followupEvals := a.createLaterEvals(
		rescheduleLater, structs.EvalTriggerAllocReschedule)
	attributeUpdates := make(allocSet)

	// Create updates that will be applied to the allocs to mark the FollowupEvalID
	for _, laterAlloc := range rescheduleLater {
		// Update the allocation if possible
		if d, ok := result.DisconnectUpdates[laterAlloc.allocID]; ok {
			d.FollowupEvalID = allocIDToFollowupEvalID[laterAlloc.alloc.ID]
		} else if m, ok := migrate[laterAlloc.allocID]; ok {
			m.FollowupEvalID = allocIDToFollowupEvalID[laterAlloc.alloc.ID]
		} else {
			// Can't update an allocation that is disconnected
			existingAlloc := all[laterAlloc.alloc.ID]
			updatedAlloc := existingAlloc.Copy()
			updatedAlloc.FollowupEvalID = allocIDToFollowupEvalID[laterAlloc.alloc.ID]
			attributeUpdates[laterAlloc.allocID] = updatedAlloc
		}
	}

	result.AttributeUpdates = attributeUpdates
	result.DesiredFollowupEvals[group] = append(
		result.DesiredFollowupEvals[group], followupEvals...)
	result.DesiredTGUpdates[group].RescheduleLater = uint64(len(rescheduleLater))
}

// appendReconnectingUpdates copies existing allocations in the unknown state,
// but whose nodes have been identified as ready. The Allocations DesiredStatus
// is set to running, and these allocs are appended to the Plan as
// non-destructive updates. Clients are responsible for reconciling the
// DesiredState with the actual state as the node comes back online.
func (a *AllocReconciler) appendReconnectingUpdates(reconnecting allocSet) allocSet {

	reconnectingUpdates := make(allocSet)

	// Create updates that will be appended to the plan.
	for _, alloc := range reconnecting {
		// If the user has defined a DesiredTransition don't resume the alloc.
		if alloc.DesiredTransition.ShouldMigrate() ||
			alloc.DesiredTransition.ShouldReschedule() ||
			alloc.DesiredTransition.ShouldForceReschedule() ||
			alloc.Job.Version < a.jobState.Job.Version ||
			alloc.Job.CreateIndex < a.jobState.Job.CreateIndex {
			continue
		}

		// If the scheduler has defined a terminal DesiredStatus don't resume the alloc.
		if alloc.DesiredStatus != structs.AllocDesiredStatusRun {
			continue
		}

		// If the alloc has failed don't reconnect.
		if alloc.ClientStatus != structs.AllocClientStatusRunning {
			continue
		}

		// Record the new ClientStatus to indicate to future evals that the
		// alloc has already reconnected.
		// Use a copy to prevent mutating the object from statestore.
		reconnectedAlloc := alloc.Copy()
		reconnectedAlloc.AppendState(structs.AllocStateFieldClientStatus, alloc.ClientStatus)
		reconnectingUpdates[reconnectedAlloc.ID] = reconnectedAlloc
	}
	return reconnectingUpdates
}

// createLaterEvals creates batched followup evaluations with the WaitUntil
// field set for lost or rescheduled allocations. returns a map of alloc IDs
// to their followupEval IDs and the list of followup evaluations.
func (a *AllocReconciler) createLaterEvals(rescheduleLater []*delayedRescheduleInfo, triggeredBy string) (map[string]string, []*structs.Evaluation) {
	if len(rescheduleLater) == 0 {
		return map[string]string{}, nil
	}

	// Sort by time
	sort.Slice(rescheduleLater, func(i, j int) bool {
		return rescheduleLater[i].rescheduleTime.Before(rescheduleLater[j].rescheduleTime)
	})

	var evals []*structs.Evaluation
	nextReschedTime := rescheduleLater[0].rescheduleTime
	allocIDToFollowupEvalID := make(map[string]string, len(rescheduleLater))

	// Create a new eval for the first batch
	eval := &structs.Evaluation{
		ID:                uuid.Generate(),
		Namespace:         a.jobState.Job.Namespace,
		Priority:          a.jobState.EvalPriority,
		Type:              a.jobState.Job.Type,
		TriggeredBy:       triggeredBy,
		JobID:             a.jobState.Job.ID,
		JobModifyIndex:    a.jobState.Job.ModifyIndex,
		Status:            structs.EvalStatusPending,
		StatusDescription: sstructs.DescReschedulingFollowupEval,
		WaitUntil:         nextReschedTime,
	}
	evals = append(evals, eval)

	for _, allocReschedInfo := range rescheduleLater {
		if allocReschedInfo.rescheduleTime.Sub(nextReschedTime) < batchedFailedAllocWindowSize {
			allocIDToFollowupEvalID[allocReschedInfo.allocID] = eval.ID
		} else {
			// Start a new batch
			nextReschedTime = allocReschedInfo.rescheduleTime
			// Create a new eval for the new batch
			eval = &structs.Evaluation{
				ID:             uuid.Generate(),
				Namespace:      a.jobState.Job.Namespace,
				Priority:       a.jobState.EvalPriority,
				Type:           a.jobState.Job.Type,
				TriggeredBy:    structs.EvalTriggerRetryFailedAlloc,
				JobID:          a.jobState.Job.ID,
				JobModifyIndex: a.jobState.Job.ModifyIndex,
				Status:         structs.EvalStatusPending,
				WaitUntil:      nextReschedTime,
			}
			evals = append(evals, eval)
			// Set the evalID for the first alloc in this new batch
			allocIDToFollowupEvalID[allocReschedInfo.allocID] = eval.ID
		}
		emitRescheduleInfo(allocReschedInfo.alloc, eval)
	}

	return allocIDToFollowupEvalID, evals
}

// createTimeoutLaterEvals creates followup evaluations with the
// WaitUntil field set for allocations in an unknown state on disconnected nodes.
// It returns a map of allocIDs to their associated followUpEvalIDs.
func (a *AllocReconciler) createTimeoutLaterEvals(disconnecting allocSet, tgName string) (map[string]string, []*structs.Evaluation) {
	if len(disconnecting) == 0 {
		return map[string]string{}, nil
	}

	timeoutDelays, err := disconnecting.delayByLostAfter(a.clusterState.Now)
	if err != nil {
		a.logger.Error("error for task_group", "task_group", tgName, "error", err)
		return map[string]string{}, nil
	}

	// Sort by time
	sort.Slice(timeoutDelays, func(i, j int) bool {
		return timeoutDelays[i].rescheduleTime.Before(timeoutDelays[j].rescheduleTime)
	})

	var evals []*structs.Evaluation
	nextReschedTime := timeoutDelays[0].rescheduleTime
	allocIDToFollowupEvalID := make(map[string]string, len(timeoutDelays))

	eval := &structs.Evaluation{
		ID:                uuid.Generate(),
		Namespace:         a.jobState.Job.Namespace,
		Priority:          a.jobState.EvalPriority,
		Type:              a.jobState.Job.Type,
		TriggeredBy:       structs.EvalTriggerMaxDisconnectTimeout,
		JobID:             a.jobState.Job.ID,
		JobModifyIndex:    a.jobState.Job.ModifyIndex,
		Status:            structs.EvalStatusPending,
		StatusDescription: sstructs.DescDisconnectTimeoutFollowupEval,
		WaitUntil:         nextReschedTime,
	}
	evals = append(evals, eval)

	// Important to remember that these are sorted. The rescheduleTime can only
	// get farther into the future. If this loop detects the next delay is greater
	// than the batch window (5s) it creates another batch.
	for _, timeoutInfo := range timeoutDelays {
		if timeoutInfo.rescheduleTime.Sub(nextReschedTime) < batchedFailedAllocWindowSize {
			allocIDToFollowupEvalID[timeoutInfo.allocID] = eval.ID
		} else {
			// Start a new batch
			nextReschedTime = timeoutInfo.rescheduleTime
			// Create a new eval for the new batch
			eval = &structs.Evaluation{
				ID:                uuid.Generate(),
				Namespace:         a.jobState.Job.Namespace,
				Priority:          a.jobState.EvalPriority,
				Type:              a.jobState.Job.Type,
				TriggeredBy:       structs.EvalTriggerMaxDisconnectTimeout,
				JobID:             a.jobState.Job.ID,
				JobModifyIndex:    a.jobState.Job.ModifyIndex,
				Status:            structs.EvalStatusPending,
				StatusDescription: sstructs.DescDisconnectTimeoutFollowupEval,
				WaitUntil:         timeoutInfo.rescheduleTime,
			}
			evals = append(evals, eval)
			allocIDToFollowupEvalID[timeoutInfo.allocID] = eval.ID
		}

		emitRescheduleInfo(timeoutInfo.alloc, eval)

	}

	return allocIDToFollowupEvalID, evals
}

// computeDisconnecting returns an allocSet disconnecting allocs that need to be
// rescheduled now, a set to reschedule later, a set of follow-up evals, and
// those allocations which can't be rescheduled.
func (a *AllocReconciler) computeDisconnecting(
	disconnecting allocSet,
	untainted, rescheduleNow *allocSet,
	rescheduleLater *[]*delayedRescheduleInfo, tg *structs.TaskGroup,
	result *ReconcileResults,
) (
	timeoutLaterEvals map[string]string,
) {
	timeoutLaterEvals = make(map[string]string)

	if tg.GetDisconnectLostAfter() != 0 {
		untaintedDisconnecting, rescheduleDisconnecting, laterDisconnecting := disconnecting.filterByRescheduleable(
			a.jobState.JobIsBatch, true, a.clusterState.Now, a.jobState.EvalID, a.jobState.DeploymentCurrent)

		*rescheduleNow = rescheduleNow.union(rescheduleDisconnecting)
		*untainted = untainted.union(untaintedDisconnecting)
		*rescheduleLater = append(*rescheduleLater, laterDisconnecting...)

		// Find delays for any disconnecting allocs that have
		// disconnect.lost_after, create followup evals, and update the
		// ClientStatus to unknown.
		var followupEvals []*structs.Evaluation
		timeoutLaterEvals, followupEvals = a.createTimeoutLaterEvals(disconnecting, tg.Name)
		result.DesiredFollowupEvals[tg.Name] = append(result.DesiredFollowupEvals[tg.Name], followupEvals...)
	}

	updates := appendUnknownDisconnectingUpdates(disconnecting, timeoutLaterEvals)
	*rescheduleNow = rescheduleNow.update(updates)

	maps.Copy(result.DisconnectUpdates, updates)
	result.DesiredTGUpdates[tg.Name].Disconnect = uint64(len(result.DisconnectUpdates))
	result.DesiredTGUpdates[tg.Name].RescheduleNow = uint64(len(*rescheduleNow))

	return timeoutLaterEvals
}

// appendUnknownDisconnectingUpdates returns a new allocSet of allocations with
// updates to mark the FollowupEvalID and and the unknown ClientStatus and
// AllocState.
func appendUnknownDisconnectingUpdates(disconnecting allocSet,
	allocIDToFollowupEvalID map[string]string) allocSet {
	resultingDisconnectUpdates := make(allocSet)
	for id, alloc := range disconnecting {
		updatedAlloc := alloc.Copy()
		updatedAlloc.ClientStatus = structs.AllocClientStatusUnknown
		updatedAlloc.AppendState(structs.AllocStateFieldClientStatus, structs.AllocClientStatusUnknown)
		updatedAlloc.ClientDescription = sstructs.StatusAllocUnknown
		updatedAlloc.FollowupEvalID = allocIDToFollowupEvalID[id]
		resultingDisconnectUpdates[updatedAlloc.ID] = updatedAlloc
	}

	return resultingDisconnectUpdates
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
