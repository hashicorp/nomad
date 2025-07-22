// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"errors"
	"slices"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

// filterAndStopAll stops all allocations in an allocSet. This is useful in when
// stopping an entire job or task group.
func filterAndStopAll(set allocSet, cs ClusterState) (uint64, []AllocStopResult) {
	untainted, migrate, lost, disconnecting, reconnecting, ignore, expiring := filterByTainted(set, cs)

	allocsToStop := slices.Concat(
		markStop(untainted, "", sstructs.StatusAllocNotNeeded),
		markStop(migrate, "", sstructs.StatusAllocNotNeeded),
		markStop(lost, structs.AllocClientStatusLost, sstructs.StatusAllocLost),
		markStop(disconnecting, "", sstructs.StatusAllocNotNeeded),
		markStop(reconnecting, "", sstructs.StatusAllocNotNeeded),
		markStop(ignore.filterByClientStatus(structs.AllocClientStatusUnknown), "", sstructs.StatusAllocNotNeeded),
		markStop(expiring.filterByClientStatus(structs.AllocClientStatusUnknown), "", sstructs.StatusAllocNotNeeded))
	return uint64(len(set)), allocsToStop
}

func filterServerTerminalAllocs(all allocSet) (remaining allocSet) {
	remaining = make(allocSet)
	for id, alloc := range all {
		if !alloc.ServerTerminalStatus() {
			remaining[id] = alloc
		}
	}
	return
}

// filterByTerminal filters out terminal allocs
func filterByTerminal(untainted allocSet) (nonTerminal allocSet) {
	nonTerminal = make(allocSet)
	for id, alloc := range untainted {
		if !alloc.TerminalStatus() {
			nonTerminal[id] = alloc
		}
	}
	return
}

// filterByDeployment filters allocations into two sets, those that match the
// given deployment ID and those that don't
func (a allocSet) filterByDeployment(id string) (match, nonmatch allocSet) {
	match = make(allocSet)
	nonmatch = make(allocSet)
	for _, alloc := range a {
		if alloc.DeploymentID == id {
			match[alloc.ID] = alloc
		} else {
			nonmatch[alloc.ID] = alloc
		}
	}
	return
}

// filterOldTerminalAllocs filters allocations that should be ignored since they
// are allocations that are terminal from a previous job version.
func filterOldTerminalAllocs(a ReconcilerState, all allocSet) (filtered, ignore allocSet) {
	if !a.JobIsBatch {
		return all, nil
	}

	filtered = filtered.union(all)
	ignored := make(allocSet)

	// Ignore terminal batch jobs from older versions
	for id, alloc := range filtered {
		older := alloc.Job.Version < a.Job.Version || alloc.Job.CreateIndex < a.Job.CreateIndex
		if older && alloc.TerminalStatus() {
			delete(filtered, id)
			ignored[id] = alloc
		}
	}

	return filtered, ignored
}

// filterByTainted takes a set of tainted nodes and filters the allocation set
// into the following groups:
// 1. Those that exist on untainted nodes
// 2. Those exist on nodes that are draining
// 3. Those that exist on lost nodes or have expired
// 4. Those that are on nodes that are disconnected, but have not had their ClientState set to unknown
// 5. Those that are on a node that has reconnected.
// 6. Those that are in a state that results in a noop.
// 7. Those that are disconnected and need to be marked lost (and possibly replaced)
func filterByTainted(a allocSet, state ClusterState) (untainted, migrate, lost, disconnecting, reconnecting, ignore, expiring allocSet) {
	untainted = make(allocSet)
	migrate = make(allocSet)
	lost = make(allocSet)
	disconnecting = make(allocSet)
	reconnecting = make(allocSet)
	ignore = make(allocSet)
	expiring = make(allocSet)

	for _, alloc := range a {
		// make sure we don't apply any reconnect logic to task groups
		// without max_client_disconnect
		supportsDisconnectedClients := alloc.SupportsDisconnectedClients(state.SupportsDisconnectedClients)

		reconnect := false

		// Only compute reconnect for unknown, running, and failed since they
		// need to go through the reconnect logic.
		if supportsDisconnectedClients &&
			(alloc.ClientStatus == structs.AllocClientStatusUnknown ||
				alloc.ClientStatus == structs.AllocClientStatusRunning ||
				alloc.ClientStatus == structs.AllocClientStatusFailed) {
			reconnect = alloc.NeedsToReconnect()
		}

		// Failed allocs that need to be reconnected must be added to
		// reconnecting so that they can be handled as a failed reconnect.
		if supportsDisconnectedClients &&
			reconnect &&
			alloc.DesiredStatus == structs.AllocDesiredStatusRun &&
			alloc.ClientStatus == structs.AllocClientStatusFailed {
			reconnecting[alloc.ID] = alloc
			continue
		}

		taintedNode, nodeIsTainted := state.TaintedNodes[alloc.NodeID]
		if taintedNode != nil && taintedNode.Status == structs.NodeStatusDisconnected {
			// Group disconnecting
			if supportsDisconnectedClients {
				// Filter running allocs on a node that is disconnected to be marked as unknown.
				if alloc.ClientStatus == structs.AllocClientStatusRunning {
					disconnecting[alloc.ID] = alloc
					continue
				}
				// Filter pending allocs on a node that is disconnected to be marked as lost.
				if alloc.ClientStatus == structs.AllocClientStatusPending {
					lost[alloc.ID] = alloc
					continue
				}

			} else {
				if alloc.PreventReplaceOnDisconnect() {
					if alloc.ClientStatus == structs.AllocClientStatusRunning {
						disconnecting[alloc.ID] = alloc
						continue
					}

					untainted[alloc.ID] = alloc
					continue
				}

				lost[alloc.ID] = alloc
				continue
			}
		}

		if alloc.TerminalStatus() && !reconnect {
			// Server-terminal allocs, if supportsDisconnectedClient and not reconnect,
			// are probably stopped replacements and should be ignored
			if supportsDisconnectedClients && alloc.ServerTerminalStatus() {
				ignore[alloc.ID] = alloc
				continue
			}

			// Terminal canaries that have been marked for migration need to be
			// migrated, otherwise we block deployments from progressing by
			// counting them as running canaries.
			if alloc.DeploymentStatus.IsCanary() && alloc.DesiredTransition.ShouldMigrate() {
				migrate[alloc.ID] = alloc
				continue
			}

			// Terminal allocs, if not reconnect, are always untainted as they
			// should never be migrated.
			untainted[alloc.ID] = alloc
			continue
		}

		// Non-terminal allocs that should migrate should always migrate
		if alloc.DesiredTransition.ShouldMigrate() {
			migrate[alloc.ID] = alloc
			continue
		}

		if supportsDisconnectedClients && alloc.Expired(state.Now) {
			expiring[alloc.ID] = alloc
			continue
		}

		// Acknowledge unknown allocs that we want to reconnect eventually.
		if supportsDisconnectedClients &&
			alloc.ClientStatus == structs.AllocClientStatusUnknown &&
			alloc.DesiredStatus == structs.AllocDesiredStatusRun {
			untainted[alloc.ID] = alloc
			continue
		}

		// Ignore failed allocs that need to be reconnected and that have been
		// marked to stop by the server.
		if supportsDisconnectedClients &&
			reconnect &&
			alloc.ClientStatus == structs.AllocClientStatusFailed &&
			alloc.DesiredStatus == structs.AllocDesiredStatusStop {
			ignore[alloc.ID] = alloc
			continue
		}

		if !nodeIsTainted || (taintedNode != nil && taintedNode.Status == structs.NodeStatusReady) {
			// Filter allocs on a node that is now re-connected to be resumed.
			if reconnect {
				// Expired unknown allocs should be processed depending on the max client disconnect
				// and/or avoid reschedule on lost configurations, they are both treated as
				// expiring.
				if alloc.Expired(state.Now) {
					expiring[alloc.ID] = alloc
					continue
				}

				reconnecting[alloc.ID] = alloc
				continue
			}

			// Otherwise, Node is untainted so alloc is untainted
			untainted[alloc.ID] = alloc
			continue
		}

		// Allocs on GC'd (nil) or lost nodes are Lost
		if taintedNode == nil {
			lost[alloc.ID] = alloc
			continue
		}

		// Allocs on terminal nodes that can't be rescheduled need to be treated
		// differently than those that can.
		if taintedNode.TerminalStatus() {
			if alloc.PreventReplaceOnDisconnect() {
				if alloc.ClientStatus == structs.AllocClientStatusUnknown {
					untainted[alloc.ID] = alloc
					continue
				} else if alloc.ClientStatus == structs.AllocClientStatusRunning {
					disconnecting[alloc.ID] = alloc
					continue
				}
			}

			lost[alloc.ID] = alloc
			continue
		}

		// All other allocs are untainted
		untainted[alloc.ID] = alloc
	}

	return
}

// filterOutByClientStatus returns a new allocSet containing allocs that don't
// have the specified client status
func (a allocSet) filterOutByClientStatus(clientStatuses ...string) allocSet {
	allocs := make(allocSet)
	for _, alloc := range a {
		if !slices.Contains(clientStatuses, alloc.ClientStatus) {
			allocs[alloc.ID] = alloc
		}
	}

	return allocs
}

// filterByClientStatus returns a new allocSet containing allocs that have the
// specified client status
func (a allocSet) filterByClientStatus(clientStatus string) allocSet {
	allocs := make(allocSet)
	for _, alloc := range a {
		if alloc.ClientStatus == clientStatus {
			allocs[alloc.ID] = alloc
		}
	}

	return allocs
}

// filterByRescheduleable filters the allocation set to return the set of
// allocations that are either untainted or a set of allocations that must
// be rescheduled now. Allocations that can be rescheduled at a future time
// are also returned so that we can create follow up evaluations for them.
// Allocs are skipped or considered untainted according to logic defined in
// shouldFilter method.
func (a allocSet) filterByRescheduleable(isBatch, isDisconnecting bool, now time.Time, evalID string, deployment *structs.Deployment) (allocSet, allocSet, []*delayedRescheduleInfo) {
	untainted := make(allocSet)
	rescheduleNow := make(allocSet)
	rescheduleLater := []*delayedRescheduleInfo{}

	for _, alloc := range a {
		// Ignore disconnecting allocs that are already unknown. This can happen
		// in the case of canaries that are interrupted by a disconnect.
		if isDisconnecting && alloc.ClientStatus == structs.AllocClientStatusUnknown {
			continue
		}

		var eligibleNow, eligibleLater bool
		var rescheduleTime time.Time

		// Ignore failing allocs that have already been rescheduled.
		// Only failed or disconnecting allocs should be rescheduled.
		// Protects against a bug allowing rescheduling running allocs.
		if alloc.NextAllocation != "" && alloc.TerminalStatus() {
			continue
		}

		isUntainted, ignore := shouldFilter(alloc, isBatch)
		if isUntainted && !isDisconnecting {
			untainted[alloc.ID] = alloc
			continue // these allocs can never be rescheduled, so skip checking
		}

		if ignore {
			continue
		}

		eligibleNow, eligibleLater, rescheduleTime = updateByReschedulable(alloc, now, evalID, deployment, isDisconnecting)
		if eligibleNow {
			rescheduleNow[alloc.ID] = alloc
			continue
		}

		// If the failed alloc is not eligible for rescheduling now we
		// add it to the untainted set.
		untainted[alloc.ID] = alloc

		if eligibleLater {
			rescheduleLater = append(rescheduleLater, &delayedRescheduleInfo{alloc.ID, alloc, rescheduleTime})
		}

	}
	return untainted, rescheduleNow, rescheduleLater
}

// shouldFilter returns whether the alloc should be ignored or considered untainted.
//
// Ignored allocs are filtered out.
// Untainted allocs count against the desired total.
// Filtering logic for batch jobs:
// If complete, and ran successfully - untainted
// If desired state is stop - ignore
//
// Filtering logic for service jobs:
// Never untainted
// If desired state is stop/evict - ignore
// If client status is complete/lost - ignore
func shouldFilter(alloc *structs.Allocation, isBatch bool) (untainted, ignore bool) {
	// Allocs from batch jobs should be filtered when the desired status
	// is terminal and the client did not finish or when the client
	// status is failed so that they will be replaced. If they are
	// complete but not failed, they shouldn't be replaced.
	if isBatch {
		switch alloc.DesiredStatus {
		case structs.AllocDesiredStatusStop:
			if alloc.RanSuccessfully() {
				return true, false
			}
			if alloc.LastRescheduleFailed() {
				return false, false
			}
			return false, true
		case structs.AllocDesiredStatusEvict:
			return false, true
		}

		switch alloc.ClientStatus {
		case structs.AllocClientStatusFailed:
			return false, false
		}

		return true, false
	}

	// Handle service jobs
	switch alloc.DesiredStatus {
	case structs.AllocDesiredStatusStop, structs.AllocDesiredStatusEvict:
		if alloc.LastRescheduleFailed() {
			return false, false
		}

		return false, true
	}

	switch alloc.ClientStatus {
	case structs.AllocClientStatusComplete, structs.AllocClientStatusLost:
		return false, true
	}

	return false, false
}

// updateByReschedulable is a helper method that encapsulates logic for whether a failed allocation
// should be rescheduled now, later or left in the untainted set
func updateByReschedulable(alloc *structs.Allocation, now time.Time, evalID string, d *structs.Deployment, isDisconnecting bool) (rescheduleNow, rescheduleLater bool, rescheduleTime time.Time) {
	// If the allocation is part of an ongoing active deployment, we only allow it to reschedule
	// if it has been marked eligible
	if d != nil && alloc.DeploymentID == d.ID && d.Active() && !alloc.DesiredTransition.ShouldReschedule() {
		return
	}

	// Check if the allocation is marked as it should be force rescheduled
	if alloc.DesiredTransition.ShouldForceReschedule() {
		rescheduleNow = true
	}

	// Reschedule if the eval ID matches the alloc's followup evalID or if its close to its reschedule time
	var eligible bool
	switch {
	case isDisconnecting:
		rescheduleTime, eligible = alloc.RescheduleTimeOnDisconnect(now)

	case alloc.ClientStatus == structs.AllocClientStatusUnknown && alloc.FollowupEvalID == evalID:
		lastDisconnectTime := alloc.LastUnknown()
		rescheduleTime, eligible = alloc.NextRescheduleTimeByTime(lastDisconnectTime)

	default:
		rescheduleTime, eligible = alloc.NextRescheduleTime()
	}

	if eligible && (alloc.FollowupEvalID == evalID || rescheduleTime.Sub(now) <= rescheduleWindowSize) {
		rescheduleNow = true
		return
	}

	if eligible && (alloc.FollowupEvalID == "" || isDisconnecting) {
		rescheduleLater = true
	}

	return
}

// delayByStopAfter returns a delay for any lost allocation that's got a
// disconnect.stop_on_client_after configured
func (a allocSet) delayByStopAfter() (later []*delayedRescheduleInfo) {
	now := time.Now().UTC()
	for _, a := range a {
		if !a.ShouldClientStop() {
			continue
		}

		t := a.WaitClientStop()

		if t.After(now) {
			later = append(later, &delayedRescheduleInfo{
				allocID:        a.ID,
				alloc:          a,
				rescheduleTime: t,
			})
		}
	}
	return later
}

// delayByLostAfter returns a delay for any unknown allocation
// that has disconnect.lost_after configured
func (a allocSet) delayByLostAfter(now time.Time) ([]*delayedRescheduleInfo, error) {
	var later []*delayedRescheduleInfo

	for _, alloc := range a {
		timeout := alloc.DisconnectTimeout(now)
		if !timeout.After(now) {
			return nil, errors.New("unable to computing disconnecting timeouts")
		}

		later = append(later, &delayedRescheduleInfo{
			allocID:        alloc.ID,
			alloc:          alloc,
			rescheduleTime: timeout,
		})
	}

	return later, nil
}
