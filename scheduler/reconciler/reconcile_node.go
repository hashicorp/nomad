// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

type NodeReconciler struct {
	DeploymentOld     *structs.Deployment
	DeploymentCurrent *structs.Deployment
	DeploymentUpdates []*structs.DeploymentStatusUpdate

	// COMPAT(1.14.0):
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

// Compute is like computeCanaryNodes however, the allocations in the
// NodeReconcileResult contain the specific nodeID they should be allocated on.
func (nr *NodeReconciler) Compute(
	job *structs.Job, // jobs whose allocations are going to be diff-ed
	readyNodes []*structs.Node, // list of nodes in the ready state
	notReadyNodes map[string]struct{}, // list of nodes in DC but not ready, e.g. draining
	taintedNodes map[string]*structs.Node, // nodes which are down or drain mode (by node id)
	infeasibleNodes map[string][]string, // maps task groups to node IDs that are not feasible for them
	live []*structs.Allocation, // non-terminal allocations
	terminal structs.TerminalByNodeByName, // latest terminal allocations (by node id)
	serverSupportsDisconnectedClients bool, // flag indicating whether to apply disconnected client logic
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

	// Canary deployments deploy to the TaskGroup.UpdateStrategy.Canary
	// percentage of eligible nodes, so we create a mapping of task group name
	// to a list of nodes that canaries should be placed on.
	canaryNodes, canariesPerTG := nr.computeCanaryNodes(required, nodeAllocs, terminal, eligibleNodes, infeasibleNodes)

	compatHadExistingDeployment := nr.DeploymentCurrent != nil

	result := new(NodeReconcileResult)
	var deploymentComplete bool
	for nodeID, allocs := range nodeAllocs {
		diff, deploymentCompleteForNode := nr.computeForNode(job, nodeID, eligibleNodes,
			notReadyNodes, taintedNodes, canaryNodes[nodeID], canariesPerTG, required,
			allocs, terminal, serverSupportsDisconnectedClients)
		result.Append(diff)

		deploymentComplete = deploymentCompleteForNode
		if deploymentComplete {
			break
		}
	}

	// COMPAT(1.14.0) prevent a new deployment from being created in the case
	// where we've upgraded the cluster while a legacy rolling deployment was in
	// flight, otherwise we won't have HealthAllocs tracking and will never mark
	// the deployment as complete
	if !compatHadExistingDeployment && nr.compatHasSameVersionAllocs {
		nr.DeploymentCurrent = nil
	}

	nr.DeploymentUpdates = append(nr.DeploymentUpdates, nr.setDeploymentStatusAndUpdates(deploymentComplete, job)...)

	return result
}

// computeCanaryNodes is a helper function that, given required task groups,
// mappings of nodes to their live allocs and terminal allocs, and a map of
// eligible nodes, outputs a map[nodeID] -> map[TG] -> bool which indicates
// which TGs this node is a canary for, and a map[TG] -> int to indicate how
// many total canaries are to be placed for a TG.
func (nr *NodeReconciler) computeCanaryNodes(required map[string]*structs.TaskGroup,
	liveAllocs map[string][]*structs.Allocation, terminalAllocs structs.TerminalByNodeByName,
	eligibleNodes map[string]*structs.Node, infeasibleNodes map[string][]string) (map[string]map[string]bool, map[string]int) {

	canaryNodes := map[string]map[string]bool{}
	eligibleNodesList := slices.Collect(maps.Values(eligibleNodes))
	canariesPerTG := map[string]int{}

	for _, tg := range required {
		if tg.Update.IsEmpty() || tg.Update.Canary == 0 {
			continue
		}

		// round up to the nearest integer
		numberOfCanaryNodes := int(math.Ceil(float64(tg.Update.Canary)*float64(len(eligibleNodes))/100)) - len(infeasibleNodes[tg.Name])

		// check if there's a current deployment present. It could be that the
		// desired amount of canaries has to be reduced due to infeasible nodes.
		// if nr.DeploymentCurrent != nil {
		// 	if dstate, ok := nr.DeploymentCurrent.TaskGroups[tg.Name]; ok {
		// 		numberOfCanaryNodes = dstate.DesiredCanaries
		// 		fmt.Printf("existing deploy, setting number of canary nodes to %v\n", dstate.DesiredCanaries)
		// 	}
		// }

		canariesPerTG[tg.Name] = numberOfCanaryNodes

		// check if there are any live allocations on any nodes that are/were
		// canaries.
		for nodeID, allocs := range liveAllocs {
			for _, a := range allocs {
				eligibleNodesList, numberOfCanaryNodes = nr.findOldCanaryNodes(
					eligibleNodesList, numberOfCanaryNodes, a, tg, canaryNodes, nodeID)
			}
		}

		// check if there are any terminal allocations that were canaries
		for nodeID, terminalAlloc := range terminalAllocs {
			for _, a := range terminalAlloc {
				eligibleNodesList, numberOfCanaryNodes = nr.findOldCanaryNodes(
					eligibleNodesList, numberOfCanaryNodes, a, tg, canaryNodes, nodeID)
			}
		}

		for i, n := range eligibleNodesList {
			// infeasible nodes can never become canary candidates
			if slices.Contains(infeasibleNodes[tg.Name], n.ID) {
				continue
			}
			if i > numberOfCanaryNodes-1 {
				break
			}

			if _, ok := canaryNodes[n.ID]; !ok {
				canaryNodes[n.ID] = map[string]bool{}
			}

			canaryNodes[n.ID][tg.Name] = true
		}
	}

	return canaryNodes, canariesPerTG
}

func (nr *NodeReconciler) findOldCanaryNodes(nodesList []*structs.Node, numberOfCanaryNodes int,
	a *structs.Allocation, tg *structs.TaskGroup, canaryNodes map[string]map[string]bool, nodeID string) ([]*structs.Node, int) {

	if a.DeploymentStatus == nil || a.DeploymentStatus.Canary == false ||
		nr.DeploymentCurrent == nil {
		return nodesList, numberOfCanaryNodes
	}

	nodes := nodesList
	numberOfCanaries := numberOfCanaryNodes
	if a.TaskGroup == tg.Name {
		if _, ok := canaryNodes[nodeID]; !ok {
			canaryNodes[nodeID] = map[string]bool{}
		}
		canaryNodes[nodeID][tg.Name] = true

		// this node should no longer be considered when searching
		// for canary nodes
		numberOfCanaries -= 1
		nodes = slices.DeleteFunc(
			nodes,
			func(n *structs.Node) bool { return n.ID == nodeID },
		)
	}
	return nodes, numberOfCanaries
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
// NodeReconcilerResult object and a boolean to indicate wither the deployment
// is complete or not.
func (nr *NodeReconciler) computeForNode(
	job *structs.Job, // job whose allocs are going to be diff-ed
	nodeID string,
	eligibleNodes map[string]*structs.Node,
	notReadyNodes map[string]struct{}, // nodes that are not ready, e.g. draining
	taintedNodes map[string]*structs.Node, // nodes which are down (by node id)
	canaryNode map[string]bool, // indicates whether this node is a canary node for tg
	canariesPerTG map[string]int, // indicates how many canary placements we expect per tg
	required map[string]*structs.TaskGroup, // set of allocations that must exist
	liveAllocs []*structs.Allocation, // non-terminal allocations that exist
	terminal structs.TerminalByNodeByName, // latest terminal allocations (by node, id)
	serverSupportsDisconnectedClients bool, // flag indicating whether to apply disconnected client logic
) (*NodeReconcileResult, bool) {
	result := new(NodeReconcileResult)

	// cancel deployments that aren't needed anymore
	var deploymentUpdates []*structs.DeploymentStatusUpdate
	nr.DeploymentOld, nr.DeploymentCurrent, deploymentUpdates = cancelUnneededDeployments(job, nr.DeploymentCurrent)
	nr.DeploymentUpdates = append(nr.DeploymentUpdates, deploymentUpdates...)

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

	// Track desired total and desired canaries across all loops
	desiredCanaries := map[string]int{}

	// Track whether we're during a canary update
	isCanarying := map[string]bool{}

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
			dstate, _ = nr.DeploymentCurrent.TaskGroups[tg.Name]
		}

		supportsDisconnectedClients := alloc.SupportsDisconnectedClients(serverSupportsDisconnectedClients)

		reconnect := false
		expired := false

		// Only compute reconnect for unknown and running since they need to go
		// through the reconnect process.
		if supportsDisconnectedClients &&
			(alloc.ClientStatus == structs.AllocClientStatusUnknown ||
				alloc.ClientStatus == structs.AllocClientStatusRunning) {
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
		if supportsDisconnectedClients && expired {
			result.Lost = append(result.Lost, AllocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     alloc,
			})
			continue
		}

		// Ignore unknown allocs that we want to reconnect eventually.
		if supportsDisconnectedClients &&
			alloc.ClientStatus == structs.AllocClientStatusUnknown &&
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
		if supportsDisconnectedClients &&
			!nodeIsTainted &&
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
				supportsDisconnectedClients &&
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
		// eligible, the diff should be ignored
		if _, ineligible := notReadyNodes[nodeID]; ineligible {
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
			if canariesPerTG[tg.Name] > 0 && dstate != nil && !dstate.Promoted {
				isCanarying[tg.Name] = true
				if canaryNode[tg.Name] {
					result.Update = append(result.Update, AllocTuple{
						Name:      name,
						TaskGroup: tg,
						Alloc:     alloc,
						Canary:    true,
					})
					desiredCanaries[tg.Name] += 1
				}
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

	// as we iterate over require groups, we'll keep track of whether the
	// deployment is complete or not
	deploymentComplete := false

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
			dstate.DesiredTotal = len(eligibleNodes)

			if isCanarying[tg.Name] && !dstate.Promoted {
				dstate.DesiredCanaries = canariesPerTG[tg.Name]
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
		deploymentComplete = nr.isDeploymentComplete(tg.Name, result, isCanarying[tg.Name])

		// check if perhaps there's nothing else to do for this TG
		if existingDeployment ||
			tg.Update.IsEmpty() ||
			(dstate.DesiredTotal == 0 && dstate.DesiredCanaries == 0) ||
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

	return result, deploymentComplete
}

func (nr *NodeReconciler) createDeployment(job *structs.Job, tg *structs.TaskGroup,
	dstate *structs.DeploymentState, updates int, allocs []*structs.Allocation, terminal map[string]*structs.Allocation) {

	// programming error
	if dstate == nil {
		return
	}

	updatingSpec := updates != 0

	hadRunning := false
	hadRunningCondition := func(a *structs.Allocation) bool {
		return a.Job.ID == job.ID && a.Job.Version == job.Version && a.Job.CreateIndex == job.CreateIndex
	}

	if slices.ContainsFunc(allocs, func(alloc *structs.Allocation) bool {
		return hadRunningCondition(alloc)
	}) {
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

func (nr *NodeReconciler) isDeploymentComplete(groupName string, buckets *NodeReconcileResult, isCanarying bool) bool {
	fmt.Printf("\n===========\n")
	fmt.Println("isDeploymentComplete call")
	complete := len(buckets.Place)+len(buckets.Migrate)+len(buckets.Update) == 0

	fmt.Printf("\nis complete? %v buckets.Place: %v buckets.Update: %v\n", complete, len(buckets.Place), len(buckets.Update))
	fmt.Printf("\nnr.deploymentCurrent == nil? %v isCanarying?: %v\n", nr.DeploymentCurrent == nil, isCanarying)
	fmt.Println("===========")

	if !complete || nr.DeploymentCurrent == nil || isCanarying {
		return false
	}

	// ensure everything is healthy
	if dstate, ok := nr.DeploymentCurrent.TaskGroups[groupName]; ok {
		fmt.Printf("\nhealthy allocs %v desiredtotal: %v desired canaries: %v\n", dstate.HealthyAllocs, dstate.DesiredTotal, dstate.DesiredCanaries)
		if dstate.HealthyAllocs < dstate.DesiredTotal { // Make sure we have enough healthy allocs
			complete = false
		}
	}

	return complete
}

func (nr *NodeReconciler) setDeploymentStatusAndUpdates(deploymentComplete bool, job *structs.Job) []*structs.DeploymentStatusUpdate {
	statusUpdates := []*structs.DeploymentStatusUpdate{}

	if d := nr.DeploymentCurrent; d != nil {

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
						DeploymentID:      nr.DeploymentCurrent.ID,
						Status:            structs.DeploymentStatusBlocked,
						StatusDescription: structs.DeploymentStatusDescriptionBlocked,
					})
				}
			} else {
				statusUpdates = append(statusUpdates, &structs.DeploymentStatusUpdate{
					DeploymentID:      nr.DeploymentCurrent.ID,
					Status:            structs.DeploymentStatusSuccessful,
					StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
				})
			}
		}

		// Mark the deployment as pending since its state is now computed.
		if d.Status == structs.DeploymentStatusInitializing {
			statusUpdates = append(statusUpdates, &structs.DeploymentStatusUpdate{
				DeploymentID:      nr.DeploymentCurrent.ID,
				Status:            structs.DeploymentStatusPending,
				StatusDescription: structs.DeploymentStatusDescriptionPendingForPeer,
			})
		}
	}

	return statusUpdates
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
