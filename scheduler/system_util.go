// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

// The structs and helpers in this file are split out of scheduler_system.go and
// shared by the system and sysbatch scheduler. No code in the generic scheduler
// or reconciler should use anything here! If you need something here for
// service/batch jobs, double-check it's safe to use for all scheduler types
// before moving it into util.go

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// materializeSystemTaskGroups is used to materialize all the task groups
// a system or sysbatch job requires.
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
	required map[string]*structs.TaskGroup, // set of allocations that must exist
	allocs []*structs.Allocation, // non-terminal allocations that exist
	terminal structs.TerminalByNodeByName, // latest terminal allocations (by node, id)
	serverSupportsDisconnectedClients bool, // flag indicating whether to apply disconnected client logic
) *diffResult {
	result := new(diffResult)

	// Scan the existing updates
	existing := make(map[string]struct{}) // set of alloc names
	for _, exist := range allocs {
		// Index the existing node
		name := exist.Name
		existing[name] = struct{}{}

		// Check for the definition in the required set
		tg, ok := required[name]

		// If not required, we stop the alloc
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

		// If we have been marked for migration and aren't terminal, migrate
		if !exist.TerminalStatus() && exist.DesiredTransition.ShouldMigrate() {
			result.migrate = append(result.migrate, allocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     exist,
			})
			continue
		}

		// If we are a sysbatch job and terminal, ignore (or stop?) the alloc
		if job.Type == structs.JobTypeSysBatch && exist.TerminalStatus() {
			result.ignore = append(result.ignore, allocTuple{
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

		// Ignore unknown allocs that we want to reconnect eventually.
		if supportsDisconnectedClients &&
			exist.ClientStatus == structs.AllocClientStatusUnknown &&
			exist.DesiredStatus == structs.AllocDesiredStatusRun {
			result.ignore = append(result.ignore, allocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     exist,
			})
			continue
		}

		node, nodeIsTainted := taintedNodes[exist.NodeID]

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
			// If the job is batch and finished successfully, the fact that the
			// node is tainted does not mean it should be migrated or marked as
			// lost as the work was already successfully finished. However for
			// service/system jobs, tasks should never complete. The check of
			// batch type, defends against client bugs.
			if exist.Job.Type == structs.JobTypeSysBatch && exist.RanSuccessfully() {
				goto IGNORE
			}

			// Filter running allocs on a node that is disconnected to be marked as unknown.
			if node != nil &&
				supportsDisconnectedClients &&
				node.Status == structs.NodeStatusDisconnected &&
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

			if !exist.TerminalStatus() && (node == nil || node.TerminalStatus()) {
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

		// Check for an existing allocation
		if _, ok := existing[name]; !ok {

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
	return result
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
	nodeAllocs := make(map[string][]*structs.Allocation, len(allocs))
	for _, alloc := range allocs {
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

	result := new(diffResult)
	for nodeID, allocs := range nodeAllocs {
		diff := diffSystemAllocsForNode(job, nodeID, eligibleNodes, notReadyNodes, taintedNodes, required, allocs, terminal, serverSupportsDisconnectedClients)
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
