package scheduler

// The structs and helpers in this file are split out of scheduler_system.go and
// shared by the system and sysbatch scheduler. No code in the generic scheduler
// or reconciler should use anything here! If you need something here for
// service/batch jobs, double-check it's safe to use for all scheduler types
// before moving it into util.go

import (
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// materializeSystemTaskGroups is used to materialize all the task groups a
// system or sysbatch job requires. Returns a map of allocation names to task
// groups.
func materializeSystemTaskGroups(job *structs.Job) map[string]*structs.TaskGroup {
	out := make(map[string]*structs.TaskGroup)
	if job.Stopped() {
		return out
	}

	for _, tg := range job.TaskGroups {
		for i := 0; i < tg.Count; i++ {
			name := fmt.Sprintf("%s.%s[%d]", job.Name, tg.Name, i)
			out[name] = tg
		}
	}
	return out
}

// diffSystemAllocsForNode is used to do a set difference between the target allocations
// and the existing allocations for a particular node. This returns 8 sets of results,
// the list of named task groups that need to be placed (no existing allocation), the
// allocations that need to be updated (job definition is newer), allocs that
// need to be migrated (node is draining), the allocs that need to be evicted
// (no longer required), those that should be ignored, those that are lost
// that need to be replaced (running on a lost node), those that are running on
// a disconnected node but may resume, and those that may still be running on
// a node that has resumed reconnected.
func diffSystemAllocsForNode(
	job *structs.Job, // job whose allocs are going to be diff-ed
	nodeID string,
	eligibleNodes map[string]*structs.Node,
	notReadyNodes map[string]struct{}, // nodes that are not ready, e.g. draining
	taintedNodes map[string]*structs.Node, // nodes which are down (by node id)
	required map[string]*structs.TaskGroup, // set of task groups (by alloc name) that must exist
	allocs []*structs.Allocation, // non-terminal allocations that exist
	terminal structs.TerminalByNodeByName, // latest terminal allocations (by node, id)
	serverSupportsDisconnectedClients bool, // flag indicating whether to apply disconnected client logic
) *diffResult {

	// Track a map of task group names (both those required and not) to
	// diffResult for allocations of that name. This lets us enforce task-group
	// global invariants before we merge all the results together for the node.
	results := map[string]*diffResult{}

	// Track the set of allocation names we've
	// seen, so we can determine if new placements are needed.
	existing := make(map[string]struct{})

	for _, exist := range allocs {
		name := exist.Name
		existing[name] = struct{}{}

		result := results[name]
		if result == nil {
			result = new(diffResult)
			results[name] = result
		}

		// Check if the allocation's task group is in the required set (it might
		// have been dropped from the jobspec). If not, we stop the alloc
		tg, ok := required[name]
		if !ok {
			result.stop = append(result.stop, allocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     exist,
			})
			continue
		}

		supportsDisconnectedClients := exist.SupportsDisconnectedClients(serverSupportsDisconnectedClients)

		reconnect := false
		expired := false

		// Only compute reconnect for unknown and running since they need to go
		// through the reconnect process.
		if supportsDisconnectedClients &&
			(exist.ClientStatus == structs.AllocClientStatusUnknown ||
				exist.ClientStatus == structs.AllocClientStatusRunning) {
			reconnect = exist.NeedsToReconnect()
			if reconnect {
				expired = exist.Expired(time.Now())
			}
		}

		// If we have been marked for migration, migrate
		if exist.DesiredTransition.ShouldMigrate() {
			result.migrate = append(result.migrate, allocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     exist,
			})
			continue
		}

		// Expired unknown allocs are lost. Expired checks that status is unknown.
		if supportsDisconnectedClients && expired {
			result.lost = append(result.lost, allocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     exist,
			})
			continue
		}

		taintedNode, nodeIsTainted := taintedNodes[exist.NodeID]

		// Ignore unknown allocs that we want to reconnect eventually.
		if supportsDisconnectedClients &&
			exist.ClientStatus == structs.AllocClientStatusUnknown &&
			exist.DesiredStatus == structs.AllocDesiredStatusRun {
			goto IGNORE
		}

		// Filter allocs on a node that is now re-connected to reconnecting.
		if supportsDisconnectedClients &&
			!nodeIsTainted &&
			reconnect {

			// Record the new ClientStatus to indicate to future evals that the
			// alloc has already reconnected.
			reconnecting := exist.Copy()
			reconnecting.AppendState(structs.AllocStateFieldClientStatus, exist.ClientStatus)
			result.reconnecting = append(result.reconnecting, allocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     reconnecting,
			})
			continue
		}

		// If we are on a tainted node, we must migrate if we are a service or
		// if the batch allocation did not finish
		if nodeIsTainted {
			// If the job is batch and finished successfully (but not yet marked
			// terminal), the fact that the node is tainted does not mean it
			// should be migrated or marked as lost as the work was already
			// successfully finished. However for service/system jobs, tasks
			// should never complete. The check of batch type, defends against
			// client bugs.
			if exist.Job.Type == structs.JobTypeSysBatch && exist.RanSuccessfully() {
				goto IGNORE
			}

			// Filter running allocs on a node that is disconnected to be marked as unknown.
			if taintedNode != nil &&
				supportsDisconnectedClients &&
				taintedNode.Status == structs.NodeStatusDisconnected &&
				exist.ClientStatus == structs.AllocClientStatusRunning {

				disconnect := exist.Copy()
				disconnect.ClientStatus = structs.AllocClientStatusUnknown
				disconnect.AppendState(structs.AllocStateFieldClientStatus, structs.AllocClientStatusUnknown)
				disconnect.ClientDescription = allocUnknown
				result.disconnecting = append(result.disconnecting, allocTuple{
					Name:      name,
					TaskGroup: tg,
					Alloc:     disconnect,
				})
				continue
			}

			if taintedNode == nil || taintedNode.TerminalStatus() {
				result.lost = append(result.lost, allocTuple{
					Name:      name,
					TaskGroup: tg,
					Alloc:     exist,
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
			result.stop = append(result.stop, allocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     exist,
			})
			continue
		}

		// If the definition is updated we need to update
		if job.JobModifyIndex != exist.Job.JobModifyIndex {
			result.update = append(result.update, allocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     exist,
			})
			continue
		}

		// Everything is up-to-date
	IGNORE:
		result.ignore = append(result.ignore, allocTuple{
			Name:      name,
			TaskGroup: tg,
			Alloc:     exist,
		})

	}

	// Scan the required groups
	for name, tg := range required {

		result := results[name]
		if result == nil {
			result = new(diffResult)
			results[name] = result
		}

		// Check for an existing allocation
		if _, ok := existing[name]; ok {

			// Assert that we don't have any extraneous allocations for this
			// task group
			count := tg.Count
			if count == 0 {
				count = 1
			}
			if len(result.ignore)+len(result.update)+len(result.reconnecting) > count {
				ensureMaxSystemAllocCount(result, count)
			}

		} else {

			// Check for a terminal sysbatch allocation, which should be not placed
			// again unless the job has been updated.
			if job.Type == structs.JobTypeSysBatch {
				if alloc, termExists := terminal.Get(nodeID, name); termExists {
					// the alloc is terminal, but now the job has been updated
					if job.JobModifyIndex != alloc.Job.JobModifyIndex {
						result.update = append(result.update, allocTuple{
							Name:      name,
							TaskGroup: tg,
							Alloc:     alloc,
						})
					} else {
						// alloc is terminal and job unchanged, leave it alone
						result.ignore = append(result.ignore, allocTuple{
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
			allocTuple := allocTuple{
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

			result.place = append(result.place, allocTuple)
		}
	}

	finalResult := new(diffResult)
	for _, result := range results {
		finalResult.Append(result)
	}

	return finalResult
}

// ensureMaxSystemAllocCount enforces the invariant that the per-node diffResult
// we have for a system or sysbatch job should never have more than "count"
// allocations in a desired-running state.
func ensureMaxSystemAllocCount(result *diffResult, count int) {

	// sort descending by JobModifyIndex, then CreateIndex. The inputs are the
	// ignore/update/reconnecting allocations for a single task group that were
	// assigned to a single node, so that constrains the size of these slices
	// and sorting won't be too expensive
	sortTuples := func(tuples []allocTuple) {
		sort.Slice(tuples, func(i, j int) bool {
			I, J := tuples[i].Alloc, tuples[j].Alloc
			if I.Job.JobModifyIndex == J.Job.JobModifyIndex {
				return I.CreateIndex > J.CreateIndex
			}
			return I.Job.JobModifyIndex > J.Job.JobModifyIndex
		})
	}

	// ignored allocs are current, so pick the most recent one and stop all the
	// rest of the running allocs on the node
	if len(result.ignore) > 0 {
		if len(result.ignore) > count {
			sortTuples(result.ignore)
			result.stop = append(result.stop, result.ignore[count:]...)
			result.ignore = result.ignore[:count]
		}
		count = count - len(result.ignore) // reduce the remaining count
		if count < 1 {
			result.stop = append(result.stop, result.update...)
			result.stop = append(result.stop, result.reconnecting...)
			result.update = []allocTuple{}
			result.reconnecting = []allocTuple{}
			return
		}
	}

	// updated allocs are for updates of the job (in-place or destructive), so
	// we can pick the most recent one and stop all the rest of the running
	// allocs on the node
	if len(result.update) > 0 {
		if len(result.update) > count {
			sortTuples(result.update)
			result.stop = append(result.stop, result.update[count:]...)
			result.update = result.update[:count]
		}
		count = count - len(result.update) // reduce the remaining count
		if count < 1 {
			result.stop = append(result.stop, result.reconnecting...)
			result.reconnecting = []allocTuple{}
			return
		}
	}

	// reconnecting allocs are for when a node reconnects after being lost with
	// running allocs. we should only see this case if we got out of sync but
	// never got a chance to eval before the node disconnected. clean up the
	// remaining mess.
	if len(result.reconnecting) > count {
		sortTuples(result.reconnecting)
		result.stop = append(result.stop, result.reconnecting[count:]...)
		result.reconnecting = result.reconnecting[:count]
	}
}

// diffSystemAllocs is like diffSystemAllocsForNode however, the allocations in the
// diffResult contain the specific nodeID they should be allocated on.
func diffSystemAllocs(
	job *structs.Job, // jobs whose allocations are going to be diff-ed
	readyNodes []*structs.Node, // list of nodes in the ready state
	notReadyNodes map[string]struct{}, // list of nodes in DC but not ready, e.g. draining
	taintedNodes map[string]*structs.Node, // nodes which are down or drain mode (by node id)
	allocs []*structs.Allocation, // non-terminal allocations
	terminal structs.TerminalByNodeByName, // latest terminal allocations (by node id)
	serverSupportsDisconnectedClients bool, // flag indicating whether to apply disconnected client logic
) *diffResult {

	// Build a mapping of nodes to all their allocs.
	allocsByNode := make(map[string][]*structs.Allocation, len(allocs))
	for _, alloc := range allocs {
		allocsByNode[alloc.NodeID] = append(allocsByNode[alloc.NodeID], alloc)
	}

	eligibleNodes := make(map[string]*structs.Node)
	for _, node := range readyNodes {
		if _, ok := allocsByNode[node.ID]; !ok {
			allocsByNode[node.ID] = nil
		}
		eligibleNodes[node.ID] = node
	}

	// Create the required task groups.
	required := materializeSystemTaskGroups(job)

	result := new(diffResult)
	for nodeID, allocsForNode := range allocsByNode {
		diff := diffSystemAllocsForNode(job, nodeID, eligibleNodes, notReadyNodes, taintedNodes, required, allocsForNode, terminal, serverSupportsDisconnectedClients)
		result.Append(diff)
	}

	return result
}

// evictAndPlace is used to mark allocations for evicts and add them to the
// placement queue. evictAndPlace modifies both the diffResult and the
// limit. It returns true if the limit has been reached.
func evictAndPlace(ctx Context, diff *diffResult, allocs []allocTuple, desc string, limit *int) bool {
	n := len(allocs)
	for i := 0; i < n && i < *limit; i++ {
		a := allocs[i]
		ctx.Plan().AppendStoppedAlloc(a.Alloc, desc, "", "")
		diff.place = append(diff.place, a)
	}
	if n <= *limit {
		*limit -= n
		return false
	}
	*limit = 0
	return true
}
