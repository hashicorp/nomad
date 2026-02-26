// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"fmt"
	"slices"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/nomad/structs"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

type NodeReconciler struct {
	DeploymentOld     *structs.Deployment
	DeploymentCurrent *structs.Deployment
	DeploymentUpdates []*structs.DeploymentStatusUpdate

	// COMPAT(1.11.0):
	// compatHasSameVersionAllocs indicates that the reconciler found some
	// allocations that were for the version being deployed
	compatHasSameVersionAllocs bool
}

func NewNodeReconciler(deployment *structs.Deployment) *NodeReconciler {
	return &NodeReconciler{
		DeploymentCurrent: deployment,
		DeploymentUpdates: make([]*structs.DeploymentStatusUpdate, 0),
	}
}

// Compute is like diffSystemAllocsForNode however, the allocations in the
// diffResult contain the specific nodeID they should be allocated on.
func (nr *NodeReconciler) Compute(
	job *structs.Job, // jobs whose allocations are going to be diff-ed
	readyNodes []*structs.Node, // list of nodes in the ready state
	notReadyNodes map[string]struct{}, // list of nodes in DC but not ready, e.g. draining
	taintedNodes map[string]*structs.Node, // nodes which are down or drain mode (by node id)
	live []*structs.Allocation, // non-terminal allocations
	terminal structs.TerminalByNodeByName, // latest terminal allocations (by node id)
) *NodeReconcileResult {

	// Build a mapping of nodes to all their allocs.
	nodeAllocs := make(map[string][]*structs.Allocation, len(live))
	for _, alloc := range live {
		nodeAllocs[alloc.NodeID] = append(nodeAllocs[alloc.NodeID], alloc)
	}

	eligibleNodes := make(map[string]*structs.Node)
	for _, node := range readyNodes {
		if _, ok := nodeAllocs[node.ID]; !ok {
			nodeAllocs[node.ID] = nil
		}
		eligibleNodes[node.ID] = node
	}

	// Create the required task groups.
	required := materializeSystemTaskGroups(job)

	compatHadExistingDeployment := nr.DeploymentCurrent != nil

	result := new(NodeReconcileResult)
	for nodeID, allocs := range nodeAllocs {
		diff := nr.computeForNode(job, nodeID, eligibleNodes,
			notReadyNodes, taintedNodes, required, allocs, terminal)
		result.Append(diff)
	}

	// COMPAT(1.11.0) prevent a new deployment from being created in the case
	// where we've upgraded the cluster while a legacy rolling deployment was in
	// flight, otherwise we won't have HealthAllocs tracking and will never mark
	// the deployment as complete
	if !compatHadExistingDeployment && nr.compatHasSameVersionAllocs {
		nr.DeploymentCurrent = nil
	}
	// COMPAT(1.11.0) prevent a new deployment from being created in the case
	// where we've upgraded the cluster while there are any older nodes in the
	// eligible set, otherwise we won't have HealthAllocs tracking and will
	// never mark the deployment as complete
	if !compatHadExistingDeployment {
		for _, node := range eligibleNodes {
			if nr.compatNodeTooOldForDeployment(node) {
				nr.DeploymentCurrent = nil
			}
		}
	}

	return result
}

// computeForNode is used to do a set difference between the target
// allocations and the existing allocations for a particular node. This returns
// 8 sets of results:
// 1. the list of named task groups that need to be placed (no existing
// allocation),
// 2. the allocations that need to be updated (job definition is newer),
// 3. allocs that need to be migrated (node is draining),
// 4. allocs that need to be evicted (no longer required),
// 5. those that should be ignored,
// 6. those that are lost that need to be replaced (running on a lost node),
// 7. those that are running on a disconnected node but may resume, and
// 8. those that may still be running on a node that has resumed reconnected.
//
// This method mutates the NodeReconciler fields, and returns a new
// NodeReconcilerResult object.
func (nr *NodeReconciler) computeForNode(
	job *structs.Job, // job whose allocs are going to be diff-ed
	nodeID string,
	eligibleNodes map[string]*structs.Node,
	notReadyNodes map[string]struct{}, // nodes that are not ready, e.g. draining
	taintedNodes map[string]*structs.Node, // nodes which are down (by node id)
	required map[string]*structs.TaskGroup, // set of allocations that must exist
	liveAllocs []*structs.Allocation, // non-terminal allocations that exist
	terminal structs.TerminalByNodeByName, // latest terminal allocations (by node, id)
) *NodeReconcileResult {
	result := new(NodeReconcileResult)

	// cancel deployments that aren't needed anymore
	nr.cancelUnnededSystemDeployments(job)

	// set deployment paused and failed, if we currently have a deployment
	var deploymentPaused, deploymentFailed bool
	if nr.DeploymentCurrent != nil {
		// deployment is paused when it's manually paused by a user, or if the
		// deployment is pending or initializing, which are the initial states
		// for multi-region job deployments.
		deploymentPaused = nr.DeploymentCurrent.Status == structs.DeploymentStatusPaused ||
			nr.DeploymentCurrent.Status == structs.DeploymentStatusPending ||
			nr.DeploymentCurrent.Status == structs.DeploymentStatusInitializing
		deploymentFailed = nr.DeploymentCurrent.Status == structs.DeploymentStatusFailed
	}

	// Scan the existing updates
	existing := make(map[string]struct{}) // set of alloc names
	for _, alloc := range liveAllocs {
		// Index the existing node
		name := alloc.Name
		existing[name] = struct{}{}

		// Check for the definition in the required set
		tg, ok := required[name]

		// If not required, we stop the alloc
		if !ok {
			result.Stop = append(result.Stop, AllocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     alloc,
			})
			continue
		}

		// populate deployment state for this task group if there is an existing
		// deployment
		var dstate = new(structs.DeploymentState)
		if nr.DeploymentCurrent != nil {
			dstate = nr.DeploymentCurrent.TaskGroups[tg.Name]
		}

		reconnect := false
		expired := false

		// Only compute reconnect for unknown and running since they need to go
		// through the reconnect process.
		if alloc.ClientStatus == structs.AllocClientStatusUnknown ||
			alloc.ClientStatus == structs.AllocClientStatusRunning {
			reconnect = alloc.NeedsToReconnect()
			if reconnect {
				expired = alloc.Expired(time.Now())
			}
		}

		// If we have been marked for migration and aren't terminal, migrate
		if alloc.DesiredTransition.ShouldMigrate() {
			result.Migrate = append(result.Migrate, AllocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     alloc,
			})
			continue
		}

		// Expired unknown allocs are lost. Expired checks that status is unknown.
		if expired {
			result.Lost = append(result.Lost, AllocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     alloc,
			})
			continue
		}

		// Ignore unknown allocs that we want to reconnect eventually.
		if alloc.ClientStatus == structs.AllocClientStatusUnknown &&
			alloc.DesiredStatus == structs.AllocDesiredStatusRun {
			result.Ignore = append(result.Ignore, AllocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     alloc,
			})
			continue
		}

		// note: the node can be both tainted and nil
		node, nodeIsTainted := taintedNodes[alloc.NodeID]

		// Filter allocs on a node that is now re-connected to reconnecting.
		if !nodeIsTainted &&
			reconnect {

			// Record the new ClientStatus to indicate to future evals that the
			// alloc has already reconnected.
			reconnecting := alloc.Copy()
			reconnecting.AppendState(structs.AllocStateFieldClientStatus, alloc.ClientStatus)
			result.Reconnecting = append(result.Reconnecting, AllocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     reconnecting,
			})
			continue
		}

		// If we are on a tainted node, we must migrate if we are a service or
		// if the batch allocation did not finish
		if nodeIsTainted {
			// If the job is batch and finished successfully, the fact that the
			// node is tainted does not mean it should be migrated or marked as
			// lost as the work was already successfully finished. However for
			// service/system jobs, tasks should never complete. The check of
			// batch type, defends against client bugs.
			if alloc.Job.Type == structs.JobTypeSysBatch && alloc.RanSuccessfully() {
				goto IGNORE
			}

			// Filter running allocs on a node that is disconnected to be marked as unknown.
			if node != nil &&
				node.Status == structs.NodeStatusDisconnected &&
				alloc.ClientStatus == structs.AllocClientStatusRunning {

				disconnect := alloc.Copy()
				disconnect.ClientStatus = structs.AllocClientStatusUnknown
				disconnect.AppendState(structs.AllocStateFieldClientStatus, structs.AllocClientStatusUnknown)
				disconnect.ClientDescription = sstructs.StatusAllocUnknown
				result.Disconnecting = append(result.Disconnecting, AllocTuple{
					Name:      name,
					TaskGroup: tg,
					Alloc:     disconnect,
				})
				continue
			}

			if node == nil || node.TerminalStatus() {
				result.Lost = append(result.Lost, AllocTuple{
					Name:      name,
					TaskGroup: tg,
					Alloc:     alloc,
				})
			} else {
				goto IGNORE
			}

			continue
		}

		// For an existing allocation, if the nodeID is no longer
		// eligible, the diff should be ignored unless the job
		// definition has been updated. If the definition has been
		// updated, stop the allocation.
		if _, ineligible := notReadyNodes[nodeID]; ineligible {
			if job.JobModifyIndex != alloc.Job.JobModifyIndex {
				result.Stop = append(result.Stop, AllocTuple{
					Name:      name,
					TaskGroup: tg,
					Alloc:     alloc,
				})

				continue
			}

			goto IGNORE
		}

		// Existing allocations on nodes that are no longer targeted
		// should be stopped
		if _, eligible := eligibleNodes[nodeID]; !eligible {
			result.Stop = append(result.Stop, AllocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     alloc,
			})
			continue
		}

		// If the definition is updated we need to update
		if job.JobModifyIndex != alloc.Job.JobModifyIndex {
			// If configured for canaries and not yet promoted, mark
			// alloc update as a canary
			if !tg.Update.IsEmpty() && tg.Update.Canary > 0 && dstate != nil && !dstate.Promoted {
				result.Update = append(result.Update, AllocTuple{
					Name:      name,
					TaskGroup: tg,
					Alloc:     alloc,
					Canary:    true,
				})
			} else {
				result.Update = append(result.Update, AllocTuple{
					Name:      name,
					TaskGroup: tg,
					Alloc:     alloc,
				})
			}
			continue
		}

		// Everything is up-to-date
	IGNORE:
		nr.compatHasSameVersionAllocs = true
		result.Ignore = append(result.Ignore, AllocTuple{
			Name:      name,
			TaskGroup: tg,
			Alloc:     alloc,
		})
	}

	// Scan the required groups
	for name, tg := range required {
		// populate deployment state for this task group
		var dstate = new(structs.DeploymentState)
		var existingDeployment bool
		if nr.DeploymentCurrent != nil {
			dstate, existingDeployment = nr.DeploymentCurrent.TaskGroups[tg.Name]
		}

		if !existingDeployment {
			dstate = &structs.DeploymentState{}
			if !tg.Update.IsEmpty() {
				dstate.AutoRevert = tg.Update.AutoRevert
				dstate.AutoPromote = tg.Update.AutoPromote
				dstate.ProgressDeadline = tg.Update.ProgressDeadline
			}
		}

		// Check for an existing allocation
		if _, ok := existing[name]; !ok {
			// Check for a terminal sysbatch allocation, which should be not placed
			// again unless the job has been updated.
			if job.Type == structs.JobTypeSysBatch {
				if alloc, termExists := terminal.Get(nodeID, name); termExists {
					// the alloc is terminal, but now the job has been updated
					if job.JobModifyIndex != alloc.Job.JobModifyIndex {
						result.Update = append(result.Update, AllocTuple{
							Name:      name,
							TaskGroup: tg,
							Alloc:     alloc,
						})
					} else {
						// alloc is terminal and job unchanged, leave it alone
						result.Ignore = append(result.Ignore, AllocTuple{
							Name:      name,
							TaskGroup: tg,
							Alloc:     alloc,
						})
					}
					continue
				}
			}

			// Require a placement if no existing allocation. If there
			// is an existing allocation, we would have checked for a potential
			// update or ignore above. Ignore placements for tainted or
			// ineligible nodes

			// Tainted and ineligible nodes for a non existing alloc
			// should be filtered out and not count towards ignore or place
			if _, tainted := taintedNodes[nodeID]; tainted {
				continue
			}
			if _, eligible := eligibleNodes[nodeID]; !eligible {
				continue
			}

			termOnNode, _ := terminal.Get(nodeID, name)
			allocTuple := AllocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     termOnNode,
			}

			// If the new allocation isn't annotated with a previous allocation
			// or if the previous allocation isn't from the same node then we
			// annotate the allocTuple with a new Allocation
			if allocTuple.Alloc == nil || allocTuple.Alloc.NodeID != nodeID {
				allocTuple.Alloc = &structs.Allocation{NodeID: nodeID}
			}

			result.Place = append(result.Place, allocTuple)
		}

		// check if deployment is place ready or complete
		deploymentPlaceReady := !deploymentPaused && !deploymentFailed

		// check if perhaps there's nothing else to do for this TG
		if existingDeployment ||
			tg.Update.IsEmpty() ||
			!deploymentPlaceReady {
			continue
		}

		maxParallel := 1
		if !tg.Update.IsEmpty() {
			maxParallel = tg.Update.MaxParallel
		}

		// maxParallel of 0 means no deployments
		if maxParallel != 0 {
			nr.createDeployment(job, tg, dstate, len(result.Update), liveAllocs, terminal[nodeID])
		}
	}

	return result
}

// cancelUnneededServiceDeployments cancels any deployment that is not needed.
// A deployment update will be staged for jobs that should stop or have the
// wrong version. Unneeded deployments include:
// 1. Jobs that are marked for stop, but there is a non-terminal deployment.
// 2. Deployments that are active, but referencing a different job version.
// 3. Deployments that are already successful.
func (nr *NodeReconciler) cancelUnnededSystemDeployments(j *structs.Job) {
	// If the job is stopped and there is a non-terminal deployment, cancel it
	if j.Stopped() {
		if nr.DeploymentCurrent != nil && nr.DeploymentCurrent.Active() {
			nr.DeploymentUpdates = append(nr.DeploymentUpdates, &structs.DeploymentStatusUpdate{
				DeploymentID:      nr.DeploymentCurrent.ID,
				Status:            structs.DeploymentStatusCancelled,
				StatusDescription: structs.DeploymentStatusDescriptionStoppedJob,
			})
		}

		// Nothing else to do
		return
	}

	if nr.DeploymentCurrent == nil {
		return
	}

	// Check if the deployment is active and referencing an older job and cancel it
	if nr.DeploymentCurrent.JobCreateIndex != j.CreateIndex || nr.DeploymentCurrent.JobVersion != j.Version {
		if nr.DeploymentCurrent.Active() {
			nr.DeploymentUpdates = append(nr.DeploymentUpdates, &structs.DeploymentStatusUpdate{
				DeploymentID:      nr.DeploymentCurrent.ID,
				Status:            structs.DeploymentStatusCancelled,
				StatusDescription: structs.DeploymentStatusDescriptionNewerJob,
			})

			nr.DeploymentOld = nr.DeploymentCurrent
			nr.DeploymentCurrent = nil
			return
		}
	}

	// Clear it as the current deployment if it is successful
	if nr.DeploymentCurrent.Status == structs.DeploymentStatusSuccessful {
		nr.DeploymentOld = nr.DeploymentCurrent
		nr.DeploymentCurrent = nil
	}
}

func (nr *NodeReconciler) createDeployment(job *structs.Job, tg *structs.TaskGroup,
	dstate *structs.DeploymentState, updates int, allocs []*structs.Allocation, terminal map[string]*structs.Allocation) {

	// programming error
	if dstate == nil {
		return
	}

	// if there's an old deployment for the same job version as we're
	// processing, never create a new one
	if nr.DeploymentOld != nil && nr.DeploymentOld.JobVersion == job.Version {
		return
	}

	updatingSpec := updates != 0

	hadRunning := false
	hadRunningCondition := func(a *structs.Allocation) bool {
		return a.Job.ID == job.ID && a.Job.Version == job.Version && a.Job.CreateIndex == job.CreateIndex
	}

	if slices.ContainsFunc(allocs, hadRunningCondition) {
		nr.compatHasSameVersionAllocs = true
		hadRunning = true
	}

	// if there's a terminal allocation it means we're doing a reschedule.
	// We don't create deployments for reschedules.
	for _, alloc := range terminal {
		if hadRunningCondition(alloc) {
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
	if nr.DeploymentCurrent == nil {
		nr.DeploymentCurrent = structs.NewDeployment(job, job.Priority, time.Now().UnixNano())
		nr.DeploymentUpdates = append(
			nr.DeploymentUpdates, &structs.DeploymentStatusUpdate{
				DeploymentID:      nr.DeploymentCurrent.ID,
				Status:            structs.DeploymentStatusRunning,
				StatusDescription: structs.DeploymentStatusDescriptionRunning,
			})
	}

	// Attach the groups deployment state to the deployment
	if nr.DeploymentCurrent.TaskGroups == nil {
		nr.DeploymentCurrent.TaskGroups = make(map[string]*structs.DeploymentState)
	}

	nr.DeploymentCurrent.TaskGroups[tg.Name] = dstate
}

var minVersionSystemDeployments = version.Must(version.NewVersion("1.11.0"))

func (nr *NodeReconciler) compatNodeTooOldForDeployment(node *structs.Node) bool {
	if node == nil {
		return false
	}
	if nodeVersionStr, ok := node.Attributes["nomad.version"]; ok {
		nodeVersion, err := version.NewVersion(nodeVersionStr)
		if err == nil && nodeVersion.LessThan(minVersionSystemDeployments) {
			return true
		}
	}
	return false
}

// materializeSystemTaskGroups is used to materialize all the task groups
// a system or sysbatch job requires.
func materializeSystemTaskGroups(job *structs.Job) map[string]*structs.TaskGroup {
	out := make(map[string]*structs.TaskGroup)
	if job.Stopped() {
		return out
	}

	for _, tg := range job.TaskGroups {
		for i := range tg.Count {
			name := fmt.Sprintf("%s.%s[%d]", job.Name, tg.Name, i)
			out[name] = tg
		}
	}
	return out
}

// AllocTuple is a tuple of the allocation name and potential alloc ID
type AllocTuple struct {
	Name      string
	TaskGroup *structs.TaskGroup
	Alloc     *structs.Allocation
	Canary    bool
}

// NodeReconcileResult is used to return the sets that result from the diff
type NodeReconcileResult struct {
	Place, Update, Migrate, Stop, Ignore, Lost, Disconnecting, Reconnecting []AllocTuple
}

func (d *NodeReconcileResult) Fields() []any {
	fields := []any{
		"ignore", len(d.Ignore),
		"place", len(d.Place),
		"update", len(d.Update),
		"stop", len(d.Stop),
		"migrate", len(d.Migrate),
		"lost", len(d.Lost),
		"disconnecting", len(d.Disconnecting),
		"reconnecting", len(d.Reconnecting),
	}

	return fields
}

func (d *NodeReconcileResult) Append(other *NodeReconcileResult) {
	d.Place = append(d.Place, other.Place...)
	d.Update = append(d.Update, other.Update...)
	d.Migrate = append(d.Migrate, other.Migrate...)
	d.Stop = append(d.Stop, other.Stop...)
	d.Ignore = append(d.Ignore, other.Ignore...)
	d.Lost = append(d.Lost, other.Lost...)
	d.Disconnecting = append(d.Disconnecting, other.Disconnecting...)
	d.Reconnecting = append(d.Reconnecting, other.Reconnecting...)
}
