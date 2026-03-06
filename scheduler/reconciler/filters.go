// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
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

// allocCategory represents the classification category for an allocation
type allocCategory string

const (
	categoryUntainted     allocCategory = "untainted"
	categoryMigrate       allocCategory = "migrate"
	categoryLost          allocCategory = "lost"
	categoryDisconnecting allocCategory = "disconnecting"
	categoryReconnecting  allocCategory = "reconnecting"
	categoryIgnore        allocCategory = "ignore"
	categoryExpiring      allocCategory = "expiring"
)

// allocContext holds all the contextual information needed to classify an allocation
type allocContext struct {
	alloc           *structs.Allocation
	shouldReconnect bool
	taintedNode     *structs.Node
	now             time.Time
}

// classificationRule defines a single decision rule in the classification table
type classificationRule struct {
	name      string
	condition func(ctx allocContext) bool
	category  allocCategory
}

// getClassificationRules returns the ordered list of classification rules.
// Rules are evaluated in order, first match wins.
// To add new cases, simply add new rules to this list in the appropriate priority order.
var classificationRules = []classificationRule{
	// Priority 1: Failed reconnect cases
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.shouldReconnect &&
				aCtx.alloc.DesiredStatus == structs.AllocDesiredStatusRun &&
				aCtx.alloc.ClientStatus == structs.AllocClientStatusFailed
		},
		category: categoryReconnecting,
	},
	// Priority 2: Server-terminal allocations (stopped replacements)
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.alloc.TerminalStatus() && !aCtx.shouldReconnect &&
				aCtx.alloc.ServerTerminalStatus()
		},
		category: categoryIgnore,
	},
	// Priority 3: Terminal canaries that need migration
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.alloc.TerminalStatus() && !aCtx.shouldReconnect &&
				aCtx.alloc.DeploymentStatus.IsCanary() &&
				aCtx.alloc.DesiredTransition.ShouldMigrate()
		},
		category: categoryMigrate,
	},
	// Priority 4: Other terminal allocations
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.alloc.TerminalStatus() && !aCtx.shouldReconnect
		},
		category: categoryUntainted,
	},
	// Priority 5: Expired allocations
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.alloc.Expired(aCtx.now)
		},
		category: categoryExpiring,
	},
	// Priority 6: Failed reconnect marked to stop
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.shouldReconnect &&
				aCtx.alloc.ClientStatus == structs.AllocClientStatusFailed &&
				aCtx.alloc.DesiredStatus == structs.AllocDesiredStatusStop
		},
		category: categoryIgnore,
	},
	// Priority 7: Disconnected node - unknown alloc
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.taintedNode != nil &&
				aCtx.taintedNode.Status == structs.NodeStatusDisconnected &&
				aCtx.alloc.ClientStatus == structs.AllocClientStatusUnknown
		},
		category: categoryUntainted,
	},
	// Priority 8: Disconnected node - pending alloc
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.taintedNode != nil &&
				aCtx.taintedNode.Status == structs.NodeStatusDisconnected &&
				aCtx.alloc.ClientStatus == structs.AllocClientStatusPending
		},
		category: categoryLost,
	},
	// Priority 9: Disconnected node - no disconnect timeout
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.taintedNode != nil &&
				aCtx.taintedNode.Status == structs.NodeStatusDisconnected &&
				aCtx.alloc.DisconnectTimeout(aCtx.now) == aCtx.now
		},
		category: categoryLost,
	},
	// Priority 10: Disconnected node - within grace period
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.taintedNode != nil &&
				aCtx.taintedNode.Status == structs.NodeStatusDisconnected
		},
		category: categoryDisconnecting,
	},
	// Priority 11: Migrate flag set
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.alloc.DesiredTransition.ShouldMigrate()
		},
		category: categoryMigrate,
	},
	// Priority 12: Untainted/ready node with reconnect
	{
		condition: func(aCtx allocContext) bool {
			return (aCtx.taintedNode == nil || (aCtx.taintedNode != nil &&
				aCtx.taintedNode.Status == structs.NodeStatusReady)) &&
				aCtx.shouldReconnect
		},
		category: categoryReconnecting,
	},
	// Priority 13: Untainted/ready node
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.taintedNode == nil || (aCtx.taintedNode != nil &&
				aCtx.taintedNode.Status == structs.NodeStatusReady)
		},
		category: categoryUntainted,
	},
	// Priority 14: Node GC'd (nil)
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.taintedNode == nil
		},
		category: categoryLost,
	},
	// Priority 15: Terminal node, no replace, unknown alloc
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.taintedNode != nil &&
				aCtx.taintedNode.TerminalStatus() &&
				!aCtx.alloc.ReplaceOnDisconnect() &&
				aCtx.alloc.ClientStatus == structs.AllocClientStatusUnknown
		},
		category: categoryUntainted,
	},
	// Priority 16: Terminal node, no replace, running alloc
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.taintedNode != nil &&
				aCtx.taintedNode.TerminalStatus() &&
				!aCtx.alloc.ReplaceOnDisconnect() &&
				aCtx.alloc.ClientStatus == structs.AllocClientStatusRunning
		},
		category: categoryDisconnecting,
	},
	// Priority 17: Terminal node (all other cases)
	{
		condition: func(aCtx allocContext) bool {
			return aCtx.taintedNode != nil && aCtx.taintedNode.TerminalStatus()
		},
		category: categoryLost,
	},
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

	// Create a map for quick category assignment
	categoryMap := map[allocCategory]*allocSet{
		categoryUntainted:     &untainted,
		categoryMigrate:       &migrate,
		categoryLost:          &lost,
		categoryDisconnecting: &disconnecting,
		categoryReconnecting:  &reconnecting,
		categoryIgnore:        &ignore,
		categoryExpiring:      &expiring,
	}

	for _, alloc := range set {
		// Build the context for classification
		ctx := allocContext{
			alloc: alloc,
			now:   state.Now,
		}

		// Compute shouldReconnect - only for unknown, running, and failed allocs
		if alloc.ClientStatus == structs.AllocClientStatusUnknown ||
			alloc.ClientStatus == structs.AllocClientStatusRunning ||
			alloc.ClientStatus == structs.AllocClientStatusFailed {
			ctx.shouldReconnect = alloc.NeedsToReconnect()
		}

		// Get node taint information
		ctx.taintedNode, _ = state.TaintedNodes[alloc.NodeID]

		// Apply classification rules in order (first match wins)
		classified := false
		for _, rule := range classificationRules {
			if rule.condition(ctx) {
				targetSet := categoryMap[rule.category]
				(*targetSet)[alloc.ID] = alloc
				classified = true
				break
			}
		}

		// Default: untainted (if no rule matched)
		if !classified {
			untainted[alloc.ID] = alloc
		}
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

	// We never want to reschedule unknown allocs that have `replace = false`
	if isDisconnecting && !alloc.ReplaceOnDisconnect() {
		return
	}

	// Reschedule if the eval ID matches the alloc's followup evalID or if its close to its reschedule time
	var eligible bool
	switch {
	case isDisconnecting:
		rescheduleTime, eligible = alloc.NextRescheduleTimeByTime(now)
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

	if eligible && alloc.FollowupEvalID == "" {
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
		later = append(later, &delayedRescheduleInfo{
			allocID:        alloc.ID,
			alloc:          alloc,
			rescheduleTime: alloc.DisconnectTimeout(now),
		})
	}

	return later, nil
}
