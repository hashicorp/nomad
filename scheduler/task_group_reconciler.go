package scheduler

import (
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

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
		}
		tgr.allocSlots[slot.name] = slot
	}
}

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

func (tgr *taskGroupReconciler) DeploymentPaused() bool {
	if tgr.deployment != nil {
		return tgr.deployment.Status == structs.DeploymentStatusPaused ||
			tgr.deployment.Status == structs.DeploymentStatusPending
	}
	return true
}

func (tgr *taskGroupReconciler) DeploymentFailed() bool {
	if tgr.deployment != nil {
		return tgr.deployment.Status == structs.DeploymentStatusFailed
	}
	// TODO (derek): what should it be if deployment nil?
	return false
}

func (tgr *taskGroupReconciler) AppendResults() {
	for _, slot := range tgr.allocSlots {
		tgr.result.stop = append(tgr.result.stop, slot.StopResults()...)
		tgr.result.place = append(tgr.result.place, slot.PlaceResults()...)
		tgr.result.destructiveUpdate = append(tgr.result.destructiveUpdate, slot.DestructiveResults()...)
		tgr.result.inplaceUpdate = append(tgr.result.inplaceUpdate, slot.InplaceUpdates()...)
		tgr.result.attributeUpdates = mergeAllocMaps(tgr.result.attributeUpdates, slot.AttributeUpdates())
		tgr.result.disconnectUpdates = mergeAllocMaps(tgr.result.disconnectUpdates, slot.DisconnectUpdates())
		tgr.result.reconnectUpdates = mergeAllocMaps(tgr.result.reconnectUpdates, slot.ReconnectUpdates())
		tgr.result.desiredFollowupEvals[tgr.taskGroupName] = append(tgr.result.desiredFollowupEvals[tgr.taskGroupName], slot.followupEvals...)
		tgr.HandleDesiredUpdates(slot)
	}

	tgr.DeploymentStatusUpdate()
	tgr.result.desiredTGUpdates[tgr.taskGroupName] = tgr.desiredUpdates
}

func (tgr *taskGroupReconciler) DeploymentStatusUpdate() {
	// TODO
}

func (tgr *taskGroupReconciler) DeploymentComplete() bool {
	return false
}

// TODO: Experiment with whether to handle along the way versus at the end.
func (tgr *taskGroupReconciler) HandleDesiredUpdates(slot *allocSlot) {
	tgr.desiredUpdates.Stop += slot.desiredUpdates.Stop
	tgr.desiredUpdates.Place += slot.desiredUpdates.Place
	tgr.desiredUpdates.DestructiveUpdate += slot.desiredUpdates.DestructiveUpdate
	tgr.desiredUpdates.InPlaceUpdate += slot.desiredUpdates.InPlaceUpdate
	tgr.desiredUpdates.Canary += slot.desiredUpdates.Canary
	tgr.desiredUpdates.Ignore += slot.desiredUpdates.Ignore
	tgr.desiredUpdates.Migrate += slot.desiredUpdates.Migrate
	tgr.desiredUpdates.Preemptions += slot.desiredUpdates.Preemptions
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
