package scheduler

import (
	"github.com/hashicorp/nomad/helper"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

// resultMediator encapsulates the set of functions alloc slots need to
// call to mutate shared state, without exposing state to each slot instance.
// This ensures only the task group reconciler can mutate shared state, and
// in predictable places. This also enables easy mocking of the task group
// reconciler for unit testing alloc slot behavior.
type resultMediator interface {
	addToStop(alloc *structs.Allocation, reason string)
	stopMigrating(alloc *structs.Allocation)
}

type taskGroupReconciler struct {
	taskGroupName string
	// logger is used to log debug information. Logging should be kept at a
	// minimal here
	logger        log.Logger
	allocUpdateFn allocUpdateType

	// job is the job being operated on, it may be nil if the job is being
	// stopped via a purge
	job *structs.Job

	// jobID is the ID of the job being operated on. The job may be nil if it is
	// being stopped, so we require this separately.
	jobID string

	// taskGroup is the taskGroup configuration extracted from the job based
	// on the passed taskGroupName
	taskGroup *structs.TaskGroup

	// isBatchJob indicates whether the job is a batch job. The job may be nil if it is
	//	// being stopped, so we require this separately.
	isBatchJob bool

	// lastDeployment is the last deployment for the job
	lastDeployment *structs.Deployment

	// deployment is the current deployment for the job
	deployment *structs.Deployment

	// taintedNodes contains a map of nodes that are tainted
	taintedNodes map[string]*structs.Node

	// existingAllocs is non-terminal existing allocations
	existingAllocs allocSet

	// evalID and evalPriority is the ID and Priority of the evaluation that
	// triggered the reconciliation.
	evalID       string
	evalPriority int

	// supportsDisconnectedClients indicates whether all servers meet the required
	// minimum version to allow application of max_client_disconnect configuration.
	supportsDisconnectedClients bool

	// now is the time used when determining rescheduling eligibility
	// defaults to time.Now, and overridden in unit tests
	now time.Time

	// DesiredUpdates is a count of changes to the reconcileResults fields. This is
	// useful for consistency checking and metrics emission.
	desiredUpdates *structs.DesiredUpdates

	// FollowUpEvals is the set of evals that need to be created based on changes
	// proposed by the reconciler. These evals are configured to trigger at some
	// future time based on the reason they are delayed. Evals may be delayed for
	// reasons such as reschedulePolicy or disconnect timeout.
	followupEvals []*structs.Evaluation

	// result is the results of the reconciliation.
	result *reconcileResults

	// allocSlots holds the set of available slots equal to the task group count
	// and are responsible for determining how to fill the slot.
	allocSlots map[string]*allocSlot

	// rescheduleNow is the set of Allocations that need to be rescheduled immediately
	// due to failure or client disconnect.
	rescheduleNow []*structs.Allocation

	// rescheduleLater is the set of Allocations that need to be rescheduled
	// at a future time. Rescheduling is delayed either due to rescheduling
	// policy or the max_client_disconnect setting.
	rescheduleLater []*delayedRescheduleInfo

	// deploymentState tracks the state of a deployment for the task group.
	deploymentState *structs.DeploymentState

	// deploymentInProgress indicates whether a deployment state for the task group
	// was found or whether this is the first pass for the deployment.
	deploymentInProgress bool
}

func ensureResultDefaults(result *reconcileResults) {
	if result.place == nil {
		result.place = []allocPlaceResult{}
	}
	if result.destructiveUpdate == nil {
		result.destructiveUpdate = []allocDestructiveResult{}
	}
	if result.inplaceUpdate == nil {
		result.inplaceUpdate = []*structs.Allocation{}
	}
	if result.stop == nil {
		result.stop = []allocStopResult{}
	}
	if result.attributeUpdates == nil {
		result.attributeUpdates = map[string]*structs.Allocation{}
	}
	if result.disconnectUpdates == nil {
		result.disconnectUpdates = map[string]*structs.Allocation{}
	}
	if result.reconnectUpdates == nil {
		result.reconnectUpdates = map[string]*structs.Allocation{}
	}
	if result.desiredTGUpdates == nil {
		result.desiredTGUpdates = map[string]*structs.DesiredUpdates{}
	}
	if result.desiredFollowupEvals == nil {
		result.desiredFollowupEvals = map[string][]*structs.Evaluation{}
	}
}

func newTaskGroupReconciler(taskGroupName string, logger log.Logger, allocUpdateFn allocUpdateType, isBatchJob bool,
	jobID string, job *structs.Job, deployment *structs.Deployment, existingAllocs allocSet,
	taintedNodes map[string]*structs.Node, evalID string, evalPriority int,
	result *reconcileResults, supportsDisconnectedClients bool) *taskGroupReconciler {

	// TODO: Add/make consistent noop guards from computeGroup
	taskGroup := job.LookupTaskGroup(taskGroupName)
	if taskGroup == nil {
		return nil
	}

	ensureResultDefaults(result)

	tgr := &taskGroupReconciler{
		taskGroupName:               taskGroupName,
		logger:                      logger.Named("task_group_reconciler"),
		allocUpdateFn:               allocUpdateFn,
		job:                         job,
		jobID:                       jobID,
		taskGroup:                   taskGroup,
		isBatchJob:                  isBatchJob,
		deployment:                  deployment.Copy(),
		existingAllocs:              existingAllocs,
		taintedNodes:                taintedNodes,
		evalID:                      evalID,
		evalPriority:                evalPriority,
		supportsDisconnectedClients: supportsDisconnectedClients,
		now:                         time.Now(),
		allocSlots:                  map[string]*allocSlot{},
		result:                      result,
		desiredUpdates:              &structs.DesiredUpdates{},
		followupEvals:               []*structs.Evaluation{},
		rescheduleNow:               []*structs.Allocation{},
		rescheduleLater:             []*delayedRescheduleInfo{},
	}

	// Set the desired updates reference for the group.
	tgr.result.desiredTGUpdates[tgr.taskGroupName] = tgr.desiredUpdates

	tgr.taskGroup = tgr.job.LookupTaskGroup(tgr.taskGroupName)
	// If the task group is nil, we can return with no further processing.
	// Later appendResults will create stop results for all existing allocs
	// and deploymentComplete will return true. A nil task groups indicates
	// the job was updated such that the task group no longer exists.
	if tgr.taskGroup == nil {
		return tgr
	}

	// TODO: See if we can do without this map
	existingAllocsBySlotName := tgr.groupAllocsBySlotName()
	tgr.buildAllocSlots(existingAllocsBySlotName)
	tgr.stopInvalidTargets(existingAllocsBySlotName)

	return tgr
}

func (tgr *taskGroupReconciler) groupAllocsBySlotName() map[string][]*structs.Allocation {
	existingAllocsBySlotName := map[string][]*structs.Allocation{}
	for _, alloc := range tgr.existingAllocs {
		if alloc.TaskGroup != tgr.taskGroup.Name {
			continue
		}
		if allocs, ok := existingAllocsBySlotName[alloc.Name]; ok {
			existingAllocsBySlotName[alloc.Name] = append(allocs, alloc)
		} else {
			existingAllocsBySlotName[alloc.Name] = []*structs.Allocation{alloc}
		}
	}
	return existingAllocsBySlotName
}

func (tgr *taskGroupReconciler) buildAllocSlots(existingAllocsBySlotName map[string][]*structs.Allocation) {
	nameIndex := newAllocNameIndex(tgr.jobID, tgr.taskGroup.Name, tgr.taskGroup.Count, tgr.existingAllocs)
	for index, name := range nameIndex.Next(uint(tgr.taskGroup.Count)) {
		slot := &allocSlot{
			name:       name,
			index:      index,
			taskGroup:  tgr.taskGroup,
			candidates: existingAllocsBySlotName[name],
			mediator:   tgr,
		}
		tgr.allocSlots[slot.name] = slot
	}
}

// TODO: Add or integrate with tests.
// stopInvalidTargets stops existing allocations that target a slot that the TaskGroup.Count no longer supports.
func (tgr *taskGroupReconciler) stopInvalidTargets(existingAllocsBySlotName map[string][]*structs.Allocation) {
	for existingSlotName, existingAllocs := range existingAllocsBySlotName {
		if _, ok := tgr.allocSlots[existingSlotName]; !ok {
			for _, existingAlloc := range existingAllocs {
				tgr.addToStop(existingAlloc, allocNotNeeded)
			}
		}
	}
}

// TODO: Add or integrate with tests.
func (tgr *taskGroupReconciler) addToStop(alloc *structs.Allocation, reason string) {
	// TODO: Is there the potential this has been mutated during processing but not persisted?
	// TODO: Do we need to check the index?
	if alloc.ClientTerminalStatus() {
		return
	}

	tgr.result.stop = append(tgr.result.stop, allocStopResult{
		alloc:             alloc,
		statusDescription: reason,
	})

	tgr.desiredUpdates.Stop++
}

// TODO: Add or integrate with tests.
func (tgr *taskGroupReconciler) stopMigrating(alloc *structs.Allocation) {
	tgr.result.stop = append(tgr.result.stop, allocStopResult{
		alloc:             alloc,
		statusDescription: allocMigrating,
	})
	tgr.desiredUpdates.Migrate++
}

func (tgr *taskGroupReconciler) deploymentPaused() bool {
	if tgr.deployment != nil {
		return tgr.deployment.Status == structs.DeploymentStatusPaused ||
			tgr.deployment.Status == structs.DeploymentStatusPending
	}
	return true
}

func (tgr *taskGroupReconciler) deploymentFailed() bool {
	if tgr.deployment != nil {
		return tgr.deployment.Status == structs.DeploymentStatusFailed
	}
	// TODO (derek): what should it be if deployment nil?
	return false
}

// TODO: Add or integrate with tests.
// TODO: Do we want to manage desiredUpdates here or along the way in each slot handler?
// appendResults iterates over alloc slots and invokes the appendResults function
// for each slot. Application of domain logic for that slot is delegated to the
// slot instance. The task group reconciler is passed by interface to each domain
// method so that management of shared state is consolidated within a single component.
// This is useful for mock testing as well.
func (tgr *taskGroupReconciler) appendResults() {
	// If the task group is nil, then the task group has been removed so all we
	// need to do is stop everything and return.
	if tgr.taskGroup == nil {
		for _, alloc := range tgr.existingAllocs {
			tgr.addToStop(alloc, allocNotNeeded)
		}
		return
	}

	tgr.initDeploymentState()

	for _, slot := range tgr.allocSlots {
		slot.resolveCandidates()
	}
}

// TODO: Add or integrate with tests.
// initDeploymentState ensures the deployment state for the current reconciliation pass
// is either initialized from state if a deployment is in progress, or creates a new one
// if this is the first pass for the deployment. When creating a new deployment state, this
// function ensures the deployment is configured to apply the task group update policy if set.
func (tgr *taskGroupReconciler) initDeploymentState() {
	if tgr.deployment != nil {
		tgr.deploymentState, tgr.deploymentInProgress = tgr.deployment.TaskGroups[tgr.taskGroupName]
	}

	if tgr.deploymentInProgress {
		return
	}

	tgr.deploymentState = &structs.DeploymentState{}

	update := tgr.taskGroup.Update
	if !update.IsEmpty() {
		tgr.deploymentState.AutoRevert = update.AutoRevert
		tgr.deploymentState.AutoPromote = update.AutoPromote
		tgr.deploymentState.ProgressDeadline = update.ProgressDeadline
	}

	return
}

// TODO: Add or integrate with tests.
// deploymentComplete inspects the current deployment state and the desired updates
// for this reconciliation pass to determine whether the deployment will be complete
// once the results from this pass are applied.
func (tgr *taskGroupReconciler) deploymentComplete() bool {
	if tgr.taskGroup == nil {
		return true
	}

	deploymentComplete := !tgr.requiresCanaries() && !tgr.requiresUpdates()

	if !deploymentComplete || tgr.deployment == nil {
		return false
	}

	// Final check to see if the deployment is deploymentComplete is to ensure everything is healthy

	if tgr.deploymentState.HealthyAllocs < helper.IntMax(tgr.deploymentState.DesiredTotal, tgr.deploymentState.DesiredCanaries) ||
		// Make sure we have enough healthy allocs
		(tgr.deploymentState.DesiredCanaries > 0 && !tgr.deploymentState.Promoted) { // Make sure we are promoted if we have canaries
		deploymentComplete = false
	}

	tgr.updateDeploymentStatus(deploymentComplete)
	// TODO: Ensure the pointer is correctly manipulated and we don't need to reset.
	// tgr.result.desiredTGUpdates[tgr.taskGroupName] = tgr.desiredUpdates

	return deploymentComplete
}

// TODO: Add or integrate with tests.
// requiresCanaries compares the task group update configuration with
// the deployment state and desired updates for this reconciliation
// pass and returns whether further canaries are needed.
func (tgr *taskGroupReconciler) requiresCanaries() bool {
	canariesPromoted := tgr.deploymentState != nil && tgr.deploymentState.Promoted

	return tgr.taskGroup.Update != nil &&
		tgr.desiredUpdates.DestructiveUpdate != 0 &&
		tgr.desiredUpdates.Canary < uint64(tgr.taskGroup.Update.Canary) &&
		!canariesPromoted
}

// TODO: Add or integrate with tests.
// requiresUpdates examines the desiredUpdates and reschedule results
// and returns whether the task group requires further updates.
func (tgr *taskGroupReconciler) requiresUpdates() bool {
	updates := tgr.desiredUpdates
	rescheduleCount := uint64(len(tgr.rescheduleNow) + len(tgr.rescheduleLater))
	return (updates.DestructiveUpdate + updates.InPlaceUpdate + updates.Place + updates.Migrate + rescheduleCount) > 0
}

// TODO: Add or integrate with tests.
// updateDeploymentStatus manages the status and status description of the current
// deployment based on the results of the current reconciliation pass.
func (tgr *taskGroupReconciler) updateDeploymentStatus(deploymentComplete bool) {
	// Mark the deployment as complete if possible
	if tgr.deployment != nil && deploymentComplete {
		if tgr.job.IsMultiregion() {
			// the unblocking/successful states come after blocked, so we
			// need to make sure we don't revert those states.
			if tgr.deployment.Status != structs.DeploymentStatusUnblocking &&
				tgr.deployment.Status != structs.DeploymentStatusSuccessful {
				tgr.result.deploymentUpdates = append(tgr.result.deploymentUpdates, &structs.DeploymentStatusUpdate{
					DeploymentID:      tgr.deployment.ID,
					Status:            structs.DeploymentStatusBlocked,
					StatusDescription: structs.DeploymentStatusDescriptionBlocked,
				})
			}
		} else {
			tgr.result.deploymentUpdates = append(tgr.result.deploymentUpdates, &structs.DeploymentStatusUpdate{
				DeploymentID:      tgr.deployment.ID,
				Status:            structs.DeploymentStatusSuccessful,
				StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
			})
		}
	}

	// Set the description of a created deployment
	if d := tgr.result.deployment; d != nil {
		if d.RequiresPromotion() {
			if d.HasAutoPromote() {
				d.StatusDescription = structs.DeploymentStatusDescriptionRunningAutoPromotion
			} else {
				d.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
			}
		}
	}
}

// TODO (derek): replace all this boilerplate with generics now that we have updated go version.
func mergeAllocMaps(m map[string]*structs.Allocation, n map[string]*structs.Allocation) map[string]*structs.Allocation {
	if len(m) == 0 && len(n) == 0 {
		return map[string]*structs.Allocation{}
	}
	if len(m) == 0 {
		return n
	}
	if len(n) == 0 {
		return m
	}

	result := copyMapStringAlloc(m)

	for k, v := range n {
		result[k] = v
	}

	return result
}

func copyMapStringAlloc(m map[string]*structs.Allocation) map[string]*structs.Allocation {
	l := len(m)
	if l == 0 {
		return nil
	}

	c := make(map[string]*structs.Allocation, l)
	for k, v := range m {
		c[k] = v
	}
	return c
}
