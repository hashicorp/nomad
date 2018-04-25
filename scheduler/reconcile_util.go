package scheduler

import (
	"fmt"
	"sort"
	"strings"

	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// placementResult is an allocation that must be placed. It potentially has a
// previous allocation attached to it that should be stopped only if the
// paired placement is complete. This gives an atomic place/stop behavior to
// prevent an impossible resource ask as part of a rolling update to wipe the
// job out.
type placementResult interface {
	// TaskGroup returns the task group the placement is for
	TaskGroup() *structs.TaskGroup

	// Name returns the name of the desired allocation
	Name() string

	// Canary returns whether the placement should be a canary
	Canary() bool

	// PreviousAllocation returns the previous allocation
	PreviousAllocation() *structs.Allocation

	// IsRescheduling returns whether the placement was rescheduling a failed allocation
	IsRescheduling() bool

	// StopPreviousAlloc returns whether the previous allocation should be
	// stopped and if so the status description.
	StopPreviousAlloc() (bool, string)
}

// allocStopResult contains the information required to stop a single allocation
type allocStopResult struct {
	alloc             *structs.Allocation
	clientStatus      string
	statusDescription string
}

// allocPlaceResult contains the information required to place a single
// allocation
type allocPlaceResult struct {
	name          string
	canary        bool
	taskGroup     *structs.TaskGroup
	previousAlloc *structs.Allocation
	reschedule    bool
}

func (a allocPlaceResult) TaskGroup() *structs.TaskGroup           { return a.taskGroup }
func (a allocPlaceResult) Name() string                            { return a.name }
func (a allocPlaceResult) Canary() bool                            { return a.canary }
func (a allocPlaceResult) PreviousAllocation() *structs.Allocation { return a.previousAlloc }
func (a allocPlaceResult) IsRescheduling() bool                    { return a.reschedule }
func (a allocPlaceResult) StopPreviousAlloc() (bool, string)       { return false, "" }

// allocDestructiveResult contains the information required to do a destructive
// update. Destructive changes should be applied atomically, as in the old alloc
// is only stopped if the new one can be placed.
type allocDestructiveResult struct {
	placeName             string
	placeTaskGroup        *structs.TaskGroup
	stopAlloc             *structs.Allocation
	stopStatusDescription string
}

func (a allocDestructiveResult) TaskGroup() *structs.TaskGroup           { return a.placeTaskGroup }
func (a allocDestructiveResult) Name() string                            { return a.placeName }
func (a allocDestructiveResult) Canary() bool                            { return false }
func (a allocDestructiveResult) PreviousAllocation() *structs.Allocation { return a.stopAlloc }
func (a allocDestructiveResult) IsRescheduling() bool                    { return false }
func (a allocDestructiveResult) StopPreviousAlloc() (bool, string) {
	return true, a.stopStatusDescription
}

// allocMatrix is a mapping of task groups to their allocation set.
type allocMatrix map[string]allocSet

// newAllocMatrix takes a job and the existing allocations for the job and
// creates an allocMatrix
func newAllocMatrix(job *structs.Job, allocs []*structs.Allocation) allocMatrix {
	m := allocMatrix(make(map[string]allocSet))
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

// allocSet is a set of allocations with a series of helper functions defined
// that help reconcile state.
type allocSet map[string]*structs.Allocation

// GoString provides a human readable view of the set
func (a allocSet) GoString() string {
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

// nameSet returns the set of allocation names
func (a allocSet) nameSet() map[string]struct{} {
	names := make(map[string]struct{}, len(a))
	for _, alloc := range a {
		names[alloc.Name] = struct{}{}
	}
	return names
}

// nameOrder returns the set of allocation names in sorted order
func (a allocSet) nameOrder() []*structs.Allocation {
	allocs := make([]*structs.Allocation, 0, len(a))
	for _, alloc := range a {
		allocs = append(allocs, alloc)
	}
	sort.Slice(allocs, func(i, j int) bool {
		return allocs[i].Index() < allocs[j].Index()
	})
	return allocs
}

// difference returns a new allocSet that has all the existing item except those
// contained within the other allocation sets
func (a allocSet) difference(others ...allocSet) allocSet {
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

// union returns a new allocSet that has the union of the two allocSets.
// Conflicts prefer the last passed allocSet containing the value
func (a allocSet) union(others ...allocSet) allocSet {
	union := make(map[string]*structs.Allocation, len(a))
	order := []allocSet{a}
	order = append(order, others...)

	for _, set := range order {
		for k, v := range set {
			union[k] = v
		}
	}

	return union
}

// fromKeys returns an alloc set matching the passed keys
func (a allocSet) fromKeys(keys ...[]string) allocSet {
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

// filterByTainted takes a set of tainted nodes and filters the allocation set
// into three groups:
// 1. Those that exist on untainted nodes
// 2. Those exist on nodes that are draining
// 3. Those that exist on lost nodes
func (a allocSet) filterByTainted(nodes map[string]*structs.Node) (untainted, migrate, lost allocSet) {
	untainted = make(map[string]*structs.Allocation)
	migrate = make(map[string]*structs.Allocation)
	lost = make(map[string]*structs.Allocation)
	for _, alloc := range a {
		// Terminal allocs are always untainted as they should never be migrated
		if alloc.TerminalStatus() {
			untainted[alloc.ID] = alloc
			continue
		}

		// Non-terminal allocs that should migrate should always migrate
		if alloc.DesiredTransition.ShouldMigrate() {
			migrate[alloc.ID] = alloc
			continue
		}

		n, ok := nodes[alloc.NodeID]
		if !ok {
			// Node is untainted so alloc is untainted
			untainted[alloc.ID] = alloc
			continue
		}

		// Allocs on GC'd (nil) or lost nodes are Lost
		if n == nil || n.TerminalStatus() {
			lost[alloc.ID] = alloc
			continue
		}

		// All other allocs are untainted
		untainted[alloc.ID] = alloc
	}
	return
}

// filterByRescheduleable filters the allocation set to return the set of allocations that are either
// untainted or a set of allocations that must be rescheduled now. Allocations that can be rescheduled
// at a future time are also returned so that we can create follow up evaluations for them. Allocs are
// skipped or considered untainted according to logic defined in shouldFilter method.
func (a allocSet) filterByRescheduleable(isBatch bool, now time.Time, evalID string, deployment *structs.Deployment) (untainted, rescheduleNow allocSet, rescheduleLater []*delayedRescheduleInfo) {
	untainted = make(map[string]*structs.Allocation)
	rescheduleNow = make(map[string]*structs.Allocation)

	for _, alloc := range a {
		var eligibleNow, eligibleLater bool
		var rescheduleTime time.Time

		// Ignore allocs that have already been rescheduled
		if alloc.NextAllocation != "" {
			continue
		}

		isUntainted, ignore := shouldFilter(alloc, isBatch)
		if isUntainted {
			untainted[alloc.ID] = alloc
		}
		if isUntainted || ignore {
			continue
		}

		// Only failed allocs with desired state run get to this point
		// If the failed alloc is not eligible for rescheduling now we add it to the untainted set
		eligibleNow, eligibleLater, rescheduleTime = updateByReschedulable(alloc, now, evalID, deployment)
		if !eligibleNow {
			untainted[alloc.ID] = alloc
			if eligibleLater {
				rescheduleLater = append(rescheduleLater, &delayedRescheduleInfo{alloc.ID, rescheduleTime})
			}
		} else {
			rescheduleNow[alloc.ID] = alloc
		}
	}
	return
}

// shouldFilter returns whether the alloc should be ignored or considered untainted
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
		case structs.AllocDesiredStatusStop, structs.AllocDesiredStatusEvict:
			if alloc.RanSuccessfully() {
				return true, false
			}
			return false, true
		default:
		}

		switch alloc.ClientStatus {
		case structs.AllocClientStatusFailed:
		default:
			return true, false
		}
		return false, false
	}

	// Handle service jobs
	switch alloc.DesiredStatus {
	case structs.AllocDesiredStatusStop, structs.AllocDesiredStatusEvict:
		return false, true
	default:
	}

	switch alloc.ClientStatus {
	case structs.AllocClientStatusComplete, structs.AllocClientStatusLost:
		return false, true
	default:
	}
	return false, false
}

// updateByReschedulable is a helper method that encapsulates logic for whether a failed allocation
// should be rescheduled now, later or left in the untainted set
func updateByReschedulable(alloc *structs.Allocation, now time.Time, evalID string, d *structs.Deployment) (rescheduleNow, rescheduleLater bool, rescheduleTime time.Time) {
	// If the allocation is part of an ongoing active deployment, we only allow it to reschedule
	// if it has been marked eligible
	if d != nil && alloc.DeploymentID == d.ID && d.Active() && !alloc.DesiredTransition.ShouldReschedule() {
		return
	}

	// Reschedule if the eval ID matches the alloc's followup evalID or if its close to its reschedule time
	rescheduleTime, eligible := alloc.NextRescheduleTime()
	if eligible && (alloc.FollowupEvalID == evalID || rescheduleTime.Sub(now) <= rescheduleWindowSize) {
		rescheduleNow = true
		return
	}
	if eligible && alloc.FollowupEvalID == "" {
		rescheduleLater = true
	}
	return
}

// filterByTerminal filters out terminal allocs
func filterByTerminal(untainted allocSet) (nonTerminal allocSet) {
	nonTerminal = make(map[string]*structs.Allocation)
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

// allocNameIndex is used to select allocation names for placement or removal
// given an existing set of placed allocations.
type allocNameIndex struct {
	job, taskGroup string
	count          int
	b              structs.Bitmap
}

// newAllocNameIndex returns an allocNameIndex for use in selecting names of
// allocations to create or stop. It takes the job and task group name, desired
// count and any existing allocations as input.
func newAllocNameIndex(job, taskGroup string, count int, in allocSet) *allocNameIndex {
	return &allocNameIndex{
		count:     count,
		b:         bitmapFrom(in, uint(count)),
		job:       job,
		taskGroup: taskGroup,
	}
}

// bitmapFrom creates a bitmap from the given allocation set and a minimum size
// maybe given. The size of the bitmap is as the larger of the passed minimum
// and the maximum alloc index of the passed input (byte aligned).
func bitmapFrom(input allocSet, minSize uint) structs.Bitmap {
	var max uint
	for _, a := range input {
		if num := a.Index(); num > max {
			max = num
		}
	}

	if l := uint(len(input)); minSize < l {
		minSize = l
	}

	if max < minSize {
		max = minSize
	} else if max%8 == 0 {
		// This may be possible if the job was scaled down. We want to make sure
		// that the max index is not byte-aligned otherwise we will overflow
		// the bitmap.
		max++
	}

	if max == 0 {
		max = 8
	}

	// byteAlign the count
	if remainder := max % 8; remainder != 0 {
		max = max + 8 - remainder
	}

	bitmap, err := structs.NewBitmap(max)
	if err != nil {
		panic(err)
	}

	for _, a := range input {
		bitmap.Set(a.Index())
	}

	return bitmap
}

// RemoveHighest removes and returns the highest n used names. The returned set
// can be less than n if there aren't n names set in the index
func (a *allocNameIndex) Highest(n uint) map[string]struct{} {
	h := make(map[string]struct{}, n)
	for i := a.b.Size(); i > uint(0) && uint(len(h)) < n; i-- {
		// Use this to avoid wrapping around b/c of the unsigned int
		idx := i - 1
		if a.b.Check(idx) {
			a.b.Unset(idx)
			h[structs.AllocName(a.job, a.taskGroup, idx)] = struct{}{}
		}
	}

	return h
}

// Set sets the indexes from the passed alloc set as used
func (a *allocNameIndex) Set(set allocSet) {
	for _, alloc := range set {
		a.b.Set(alloc.Index())
	}
}

// Unset unsets all indexes of the passed alloc set as being used
func (a *allocNameIndex) Unset(as allocSet) {
	for _, alloc := range as {
		a.b.Unset(alloc.Index())
	}
}

// UnsetIndex unsets the index as having its name used
func (a *allocNameIndex) UnsetIndex(idx uint) {
	a.b.Unset(idx)
}

// NextCanaries returns the next n names for use as canaries and sets them as
// used. The existing canaries and destructive updates are also passed in.
func (a *allocNameIndex) NextCanaries(n uint, existing, destructive allocSet) []string {
	next := make([]string, 0, n)

	// Create a name index
	existingNames := existing.nameSet()

	// First select indexes from the allocations that are undergoing destructive
	// updates. This way we avoid duplicate names as they will get replaced.
	dmap := bitmapFrom(destructive, uint(a.count))
	remainder := n
	for _, idx := range dmap.IndexesInRange(true, uint(0), uint(a.count)-1) {
		name := structs.AllocName(a.job, a.taskGroup, uint(idx))
		if _, used := existingNames[name]; !used {
			next = append(next, name)
			a.b.Set(uint(idx))

			// If we have enough, return
			remainder = n - uint(len(next))
			if remainder == 0 {
				return next
			}
		}
	}

	// Get the set of unset names that can be used
	for _, idx := range a.b.IndexesInRange(false, uint(0), uint(a.count)-1) {
		name := structs.AllocName(a.job, a.taskGroup, uint(idx))
		if _, used := existingNames[name]; !used {
			next = append(next, name)
			a.b.Set(uint(idx))

			// If we have enough, return
			remainder = n - uint(len(next))
			if remainder == 0 {
				return next
			}
		}
	}

	// We have exhausted the preferred and free set. Pick starting from n to
	// n+remainder, to avoid overlapping where possible. An example is the
	// desired count is 3 and we want 5 canaries. The first 3 canaries can use
	// index [0, 1, 2] but after that we prefer picking indexes [4, 5] so that
	// we do not overlap. Once the canaries are promoted, these would be the
	// allocations that would be shut down as well.
	for i := uint(a.count); i < uint(a.count)+remainder; i++ {
		name := structs.AllocName(a.job, a.taskGroup, i)
		next = append(next, name)
	}

	return next
}

// Next returns the next n names for use as new placements and sets them as
// used.
func (a *allocNameIndex) Next(n uint) []string {
	next := make([]string, 0, n)

	// Get the set of unset names that can be used
	remainder := n
	for _, idx := range a.b.IndexesInRange(false, uint(0), uint(a.count)-1) {
		next = append(next, structs.AllocName(a.job, a.taskGroup, uint(idx)))
		a.b.Set(uint(idx))

		// If we have enough, return
		remainder = n - uint(len(next))
		if remainder == 0 {
			return next
		}
	}

	// We have exhausted the free set, now just pick overlapping indexes
	var i uint
	for i = 0; i < remainder; i++ {
		next = append(next, structs.AllocName(a.job, a.taskGroup, i))
		a.b.Set(i)
	}

	return next
}
