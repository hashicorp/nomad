// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package alloc

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// rescheduleWindowSize is the window size relative to current time within
// which reschedulable allocations are placed. This helps protect against small
// clock drifts between servers.
const rescheduleWindowSize = 1 * time.Second

// DelayedRescheduleInfo contains the allocation id and a time when its
// eligible to be rescheduled. This is used to create follow-up evaluations.
type DelayedRescheduleInfo struct {

	// AllocID is the ID of the allocation eligible to be rescheduled
	AllocID string

	Alloc *structs.Allocation

	// RescheduleTime is the time to use in the delayed evaluation
	RescheduleTime time.Time
}

// Matrix is a mapping of task groups to their allocation set.
type Matrix map[string]Set

// NewMatrix takes a job and the existing allocations for the job and
// creates an allocMatrix
func NewMatrix(job *structs.Job, allocs []*structs.Allocation) Matrix {
	m := Matrix(make(map[string]Set))
	for _, a := range allocs {
		s, ok := m[a.TaskGroup]
		if !ok {
			s = make(map[string]*structs.Allocation)
			m[a.TaskGroup] = s
		}
		s[a.ID] = a
	}

	if job != nil {
		for _, tg := range job.TaskGroups {
			if _, ok := m[tg.Name]; !ok {
				m[tg.Name] = make(map[string]*structs.Allocation)
			}
		}
	}
	return m
}

// Set is a set of allocations with a series of helper functions defined
// that help reconcile state.
type Set map[string]*structs.Allocation

// GoString provides a human readable view of the set
func (a Set) GoString() string {
	if len(a) == 0 {
		return "[]"
	}

	start := fmt.Sprintf("len(%d) [\n", len(a))
	var s []string
	for k, v := range a {
		s = append(s, fmt.Sprintf("%q: %v", k, v.Name))
	}
	return start + strings.Join(s, "\n") + "]"
}

// NameSet returns the set of allocation names
func (a Set) NameSet() map[string]struct{} {
	names := make(map[string]struct{}, len(a))
	for _, alloc := range a {
		names[alloc.Name] = struct{}{}
	}
	return names
}

// NameOrder returns the set of allocation names in sorted order
func (a Set) NameOrder() []*structs.Allocation {
	allocs := make([]*structs.Allocation, 0, len(a))
	for _, alloc := range a {
		allocs = append(allocs, alloc)
	}
	sort.Slice(allocs, func(i, j int) bool {
		return allocs[i].Index() < allocs[j].Index()
	})
	return allocs
}

// Difference returns a new allocSet that has all the existing item except
// those contained within the other allocation sets.
func (a Set) Difference(others ...Set) Set {
	diff := make(map[string]*structs.Allocation)
OUTER:
	for k, v := range a {
		for _, other := range others {
			if _, ok := other[k]; ok {
				continue OUTER
			}
		}
		diff[k] = v
	}
	return diff
}

// Union returns a new allocSet that has the union of the two allocSets.
// Conflicts prefer the last passed allocSet containing the value
func (a Set) Union(others ...Set) Set {
	union := make(map[string]*structs.Allocation, len(a))
	order := []Set{a}
	order = append(order, others...)

	for _, set := range order {
		for k, v := range set {
			union[k] = v
		}
	}

	return union
}

// FromKeys returns an alloc set matching the passed keys
func (a Set) FromKeys(keys ...[]string) Set {
	from := make(map[string]*structs.Allocation)
	for _, set := range keys {
		for _, k := range set {
			if alloc, ok := a[k]; ok {
				from[k] = alloc
			}
		}
	}
	return from
}

// FilterByTainted takes a set of tainted nodes and filters the allocation set
// into the following groups:
// 1. Those that exist on untainted nodes
// 2. Those exist on nodes that are draining
// 3. Those that exist on lost nodes or have expired
// 4. Those that are on nodes that are disconnected, but have not had their ClientState set to unknown
// 5. Those that are on a node that has reconnected.
// 6. Those that are in a state that results in a noop.
func (a Set) FilterByTainted(taintedNodes map[string]*structs.Node, serverSupportsDisconnectedClients bool, now time.Time) (untainted, migrate, lost, disconnecting, reconnecting, ignore Set) {
	untainted = make(map[string]*structs.Allocation)
	migrate = make(map[string]*structs.Allocation)
	lost = make(map[string]*structs.Allocation)
	disconnecting = make(map[string]*structs.Allocation)
	reconnecting = make(map[string]*structs.Allocation)
	ignore = make(map[string]*structs.Allocation)

	for _, alloc := range a {
		// make sure we don't apply any reconnect logic to task groups
		// without max_client_disconnect
		supportsDisconnectedClients := alloc.SupportsDisconnectedClients(serverSupportsDisconnectedClients)

		reconnect := false
		expired := false

		// Only compute reconnect for unknown, running, and failed since they
		// need to go through the reconnect logic.
		if supportsDisconnectedClients &&
			(alloc.ClientStatus == structs.AllocClientStatusUnknown ||
				alloc.ClientStatus == structs.AllocClientStatusRunning ||
				alloc.ClientStatus == structs.AllocClientStatusFailed) {
			reconnect = alloc.NeedsToReconnect()
			if reconnect {
				expired = alloc.Expired(now)
			}
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

		taintedNode, nodeIsTainted := taintedNodes[alloc.NodeID]
		if taintedNode != nil {
			// Group disconnecting
			switch taintedNode.Status {
			case structs.NodeStatusDisconnected:
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
					lost[alloc.ID] = alloc
					continue
				}
			case structs.NodeStatusReady:
				// Filter reconnecting allocs on a node that is now connected.
				if reconnect {
					if expired {
						lost[alloc.ID] = alloc
						continue
					}

					reconnecting[alloc.ID] = alloc
					continue
				}
			default:
			}
		}

		if alloc.TerminalStatus() && !reconnect {
			// Terminal allocs, if supportsDisconnectedClient and not reconnect,
			// are probably stopped replacements and should be ignored
			if supportsDisconnectedClients {
				ignore[alloc.ID] = alloc
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

		// Expired unknown allocs are lost
		if supportsDisconnectedClients && alloc.Expired(now) {
			lost[alloc.ID] = alloc
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

		if !nodeIsTainted {
			// Filter allocs on a node that is now re-connected to be resumed.
			if reconnect {
				if expired {
					lost[alloc.ID] = alloc
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
		if taintedNode == nil || taintedNode.TerminalStatus() {
			lost[alloc.ID] = alloc
			continue
		}

		// All other allocs are untainted
		untainted[alloc.ID] = alloc
	}

	return
}

// FilterByRescheduleable filters the allocation set to return the set of allocations that are either
// untainted or a set of allocations that must be rescheduled now. Allocations that can be rescheduled
// at a future time are also returned so that we can create follow up evaluations for them. Allocs are
// skipped or considered untainted according to logic defined in shouldFilter method.
func (a Set) FilterByRescheduleable(isBatch, isDisconnecting bool, now time.Time, evalID string, deployment *structs.Deployment) (Set, Set, []*DelayedRescheduleInfo) {
	untainted := make(map[string]*structs.Allocation)
	rescheduleNow := make(map[string]*structs.Allocation)
	rescheduleLater := []*DelayedRescheduleInfo{}

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
			rescheduleLater = append(rescheduleLater, &DelayedRescheduleInfo{alloc.ID, alloc, rescheduleTime})
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

	if eligible && (alloc.FollowupEvalID == "" || isDisconnecting) {
		rescheduleLater = true
	}

	return
}

// FilterByTerminal filters out terminal allocs
func FilterByTerminal(untainted Set) (nonTerminal Set) {
	nonTerminal = make(map[string]*structs.Allocation)
	for id, alloc := range untainted {
		if !alloc.TerminalStatus() {
			nonTerminal[id] = alloc
		}
	}
	return
}

// FilterByDeployment filters allocations into two sets, those that match the
// given deployment ID and those that don't
func (a Set) FilterByDeployment(id string) (match, nonmatch Set) {
	match = make(map[string]*structs.Allocation)
	nonmatch = make(map[string]*structs.Allocation)
	for _, alloc := range a {
		if alloc.DeploymentID == id {
			match[alloc.ID] = alloc
		} else {
			nonmatch[alloc.ID] = alloc
		}
	}
	return
}

// DelayByStopAfterClientDisconnect returns a delay for any lost allocation that's got a
// stop_after_client_disconnect configured
func (a Set) DelayByStopAfterClientDisconnect() (later []*DelayedRescheduleInfo) {
	now := time.Now().UTC()
	for _, a := range a {
		if !a.ShouldClientStop() {
			continue
		}

		t := a.WaitClientStop()

		if t.After(now) {
			later = append(later, &DelayedRescheduleInfo{
				AllocID:        a.ID,
				Alloc:          a,
				RescheduleTime: t,
			})
		}
	}
	return later
}

// DelayByMaxClientDisconnect returns a delay for any unknown allocation
// that's got a max_client_reconnect configured
func (a Set) DelayByMaxClientDisconnect(now time.Time) ([]*DelayedRescheduleInfo, error) {
	var later []*DelayedRescheduleInfo

	for _, alloc := range a {
		timeout := alloc.DisconnectTimeout(now)
		if !timeout.After(now) {
			return nil, errors.New("unable to computing disconnecting timeouts")
		}

		later = append(later, &DelayedRescheduleInfo{
			AllocID:        alloc.ID,
			Alloc:          alloc,
			RescheduleTime: timeout,
		})
	}

	return later, nil
}

// FilterOutByClientStatus returns all allocs from the set without the specified client status.
func (a Set) FilterOutByClientStatus(clientStatus string) Set {
	allocs := make(Set)
	for _, alloc := range a {
		if alloc.ClientStatus != clientStatus {
			allocs[alloc.ID] = alloc
		}
	}

	return allocs
}

// FilterByClientStatus returns allocs from the set with the specified client status.
func (a Set) FilterByClientStatus(clientStatus string) Set {
	allocs := make(Set)
	for _, alloc := range a {
		if alloc.ClientStatus == clientStatus {
			allocs[alloc.ID] = alloc
		}
	}

	return allocs
}
