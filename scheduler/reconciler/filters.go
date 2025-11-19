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

// filterAndStopAll returns a stop result including all allocations in the
// allocSet. This is useful in when stopping an entire job or task group.
func (set allocSet) filterAndStopAll(cs ClusterState) (uint64, []AllocStopResult) {
	untainted, migrate, lost, disconnecting, reconnecting, ignore, expiring := set.filterByTainted(cs)

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

// filterServerTerminalAllocs returns a new allocSet that includes only
// non-server-terminal allocations, and batch job allocs that are not marked
// for rescheduling.
//
// NOTE: Batch jobs need to always be included so the placements can
// be computed correctly (completed allocations are not replaced). For
// batch job allocations found, any that have been marked to be
// rescheduled should be filtered. If those allocations are not filtered,
// they will be used within the total count for the batch allocations and
// will result in batch allocations stopped with the `alloc stop` command
// not being properly placed after rescheduling.
func (set allocSet) filterServerTerminalAllocs() (remaining allocSet) {
	remaining = make(allocSet)
	for id, alloc := range set {
		// check if the allocation is non-server-terminal or if it is a
		// batch job allocation that has not been marked for rescheduling.
		if !alloc.ServerTerminalStatus() || (alloc.Job.Type == structs.JobTypeBatch && !alloc.DesiredTransition.ShouldReschedule()) {
			remaining[id] = alloc
		}
	}
	return
}

// filterByTerminal returns a new allocSet without any terminal allocations.
func (set allocSet) filterByTerminal() (nonTerminal allocSet) {
	nonTerminal = make(allocSet)
	for id, alloc := range set {
		if !alloc.TerminalStatus() {
			nonTerminal[id] = alloc
		}
	}
	return
}

// filterByDeployment returns two new allocSets: those allocations that match the
// given deployment ID and those that don't.
func (set allocSet) filterByDeployment(id string) (match, nonmatch allocSet) {
	match = make(allocSet)
	nonmatch = make(allocSet)
	for _, alloc := range set {
		if alloc.DeploymentID == id {
			match[alloc.ID] = alloc
		} else {
			nonmatch[alloc.ID] = alloc
		}
	}
	return
}

// filterOldTerminalAllocs returns two new allocSets: those that should be
// ignored because they are terminal from a previous job version (second) and
// any remaining (first).
func (set allocSet) filterOldTerminalAllocs(a ReconcilerState) (remain, ignore allocSet) {
	if !a.JobIsBatch {
		return set, nil
	}

	remain = remain.union(set)
	ignored := make(allocSet)

	// Ignore terminal batch jobs from older versions
	for id, alloc := range remain {
		older := alloc.Job.Version < a.Job.Version || alloc.Job.CreateIndex < a.Job.CreateIndex
		if older && alloc.TerminalStatus() {
			delete(remain, id)
			ignored[id] = alloc
		}
	}

	return remain, ignored
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
func (set allocSet) filterByTainted(state ClusterState) (untainted, migrate, lost, disconnecting, reconnecting, ignore, expiring allocSet) {
	untainted = make(allocSet)
	migrate = make(allocSet)
	lost = make(allocSet)
	disconnecting = make(allocSet)
	reconnecting = make(allocSet)
	ignore = make(allocSet)
	expiring = make(allocSet)

	for _, alloc := range set {
		shouldReconnect := false

		// Only compute reconnect for unknown, running, and failed since they
		// need to go through the reconnect logic.
		if alloc.ClientStatus == structs.AllocClientStatusUnknown ||
			alloc.ClientStatus == structs.AllocClientStatusRunning ||
			alloc.ClientStatus == structs.AllocClientStatusFailed {
			shouldReconnect = alloc.NeedsToReconnect() && state.SupportsDisconnectedClients
		}

		// Failed allocs that need to be reconnected must be added to
		// reconnecting so that they can be handled as a failed reconnect.
		if shouldReconnect &&
			alloc.DesiredStatus == structs.AllocDesiredStatusRun &&
			alloc.ClientStatus == structs.AllocClientStatusFailed {
			reconnecting[alloc.ID] = alloc
			continue
		}

		if alloc.TerminalStatus() && !shouldReconnect {
			// Server-terminal allocs, if they shouldn't reconnect,
			// are probably stopped replacements and should be ignored
			if alloc.ServerTerminalStatus() {
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

		// Expired allocs should be processed depending on the disconnect.lost_after
		// and/or avoid reschedule on lost configurations, they are both treated as
		// expiring.
		if alloc.Expired(state.Now) {
			expiring[alloc.ID] = alloc
			continue
		}

		taintedNode, nodeIsTainted := state.TaintedNodes[alloc.NodeID]
		if taintedNode != nil && taintedNode.Status == structs.NodeStatusDisconnected {
			if !state.SupportsDisconnectedClients {
				lost[alloc.ID] = alloc
				continue
			}
			// Acknowledge unknown allocs that we want to reconnect eventually.
			if alloc.ClientStatus == structs.AllocClientStatusUnknown {
				untainted[alloc.ID] = alloc
				continue
			}
			// if the alloc shouldn't be replaced, mark it disconnecting, it won't get a followup eval
			if !alloc.ReplaceOnDisconnect() {
				disconnecting[alloc.ID] = alloc
				continue
			}
			// If the alloc has a lost timeout, mark it disconnecting, it will get a followup eval later.
			// Only mark running allocs as disconnected, any other status will get immediately replaced
			if alloc.DisconnectLostAfter() != 0 && alloc.ClientStatus == structs.AllocClientStatusRunning {
				disconnecting[alloc.ID] = alloc
				continue
			}

			// If the alloc is pending or has no disconnect.lost_after set, mark it lost so it is replaced.
			if alloc.ClientStatus == structs.AllocClientStatusPending || alloc.DisconnectLostAfter() == 0 {
				lost[alloc.ID] = alloc
				continue
			}
		}

		// Non-terminal allocs that should migrate should always migrate
		if alloc.DesiredTransition.ShouldMigrate() {
			migrate[alloc.ID] = alloc
			continue
		}

		// Ignore failed allocs that need to be reconnected and that have been
		// marked to stop by the server.
		if shouldReconnect &&
			alloc.ClientStatus == structs.AllocClientStatusFailed &&
			alloc.DesiredStatus == structs.AllocDesiredStatusStop {
			ignore[alloc.ID] = alloc
			continue
		}

		if !nodeIsTainted || (taintedNode != nil && taintedNode.Status == structs.NodeStatusReady) {
			// Filter allocs on a node that is now re-connected to be resumed.
			if shouldReconnect {
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
			if !alloc.ReplaceOnDisconnect() {
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
func (set allocSet) filterOutByClientStatus(clientStatuses ...string) allocSet {
	allocs := make(allocSet)
	for _, alloc := range set {
		if !slices.Contains(clientStatuses, alloc.ClientStatus) {
			allocs[alloc.ID] = alloc
		}
	}

	return allocs
}

// filterByClientStatus returns a new allocSet containing allocs that have the
// specified client status
func (set allocSet) filterByClientStatus(clientStatus string) allocSet {
	allocs := make(allocSet)
	for _, alloc := range set {
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
func (set allocSet) filterByRescheduleable(isBatch, isDisconnecting bool,
	now time.Time, evalID string, deployment *structs.Deployment,
) (
	untainted, rescheduleNow allocSet, rescheduleLater []*delayedRescheduleInfo,
) {
	untainted = make(allocSet)
	rescheduleNow = make(allocSet)
	rescheduleLater = []*delayedRescheduleInfo{}

	for _, alloc := range set {
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
		// if the batch job allocation is flagged for being rescheduled,
		// which happens when stopped with the `alloc stop` command, the
		// allocation should not be untainted nor ignored.
		if alloc.DesiredTransition.ShouldReschedule() {
			return false, false
		}

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
func (set allocSet) delayByStopAfter() (later []*delayedRescheduleInfo) {
	now := time.Now().UTC()
	for _, a := range set {
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
func (set allocSet) delayByLostAfter(now time.Time) ([]*delayedRescheduleInfo, error) {
	var later []*delayedRescheduleInfo

	for _, alloc := range set {
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
