// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// This file contains various types and methods that are used for keeping track
// of allocations during reconciliation process.

// PlacementResult is an allocation that must be placed. It potentially has a
// previous allocation attached to it that should be stopped only if the
// paired placement is complete. This gives an atomic place/stop behavior to
// prevent an impossible resource ask as part of a rolling update to wipe the
// job out.
type PlacementResult interface {
	// TaskGroup returns the task group the placement is for
	TaskGroup() *structs.TaskGroup

	// Name returns the name of the desired allocation
	Name() string

	// Canary returns whether the placement should be a canary
	Canary() bool

	// PreviousAllocation returns the previous allocation
	PreviousAllocation() *structs.Allocation

	// SetPreviousAllocation updates the reference to the previous allocation
	SetPreviousAllocation(*structs.Allocation)

	// IsRescheduling returns whether the placement was rescheduling a failed allocation
	IsRescheduling() bool

	// StopPreviousAlloc returns whether the previous allocation should be
	// stopped and if so the status description.
	StopPreviousAlloc() (bool, string)

	// PreviousLost is true if the previous allocation was lost.
	PreviousLost() bool

	// DowngradeNonCanary indicates that placement should use the latest stable job
	// with the MinJobVersion, rather than the current deployment version
	DowngradeNonCanary() bool

	MinJobVersion() uint64
}

// AllocStopResult contains the information required to stop a single allocation
type AllocStopResult struct {
	Alloc             *structs.Allocation
	ClientStatus      string
	StatusDescription string
	FollowupEvalID    string
}

// AllocPlaceResult contains the information required to place a single
// allocation
type AllocPlaceResult struct {
	name          string
	canary        bool
	taskGroup     *structs.TaskGroup
	previousAlloc *structs.Allocation
	reschedule    bool
	lost          bool

	downgradeNonCanary bool
	minJobVersion      uint64
}

func (a AllocPlaceResult) TaskGroup() *structs.TaskGroup           { return a.taskGroup }
func (a AllocPlaceResult) Name() string                            { return a.name }
func (a AllocPlaceResult) Canary() bool                            { return a.canary }
func (a AllocPlaceResult) PreviousAllocation() *structs.Allocation { return a.previousAlloc }
func (a AllocPlaceResult) SetPreviousAllocation(alloc *structs.Allocation) {
	a.previousAlloc = alloc
}
func (a AllocPlaceResult) IsRescheduling() bool                { return a.reschedule }
func (a AllocPlaceResult) StopPreviousAlloc() (bool, string)   { return false, "" }
func (a AllocPlaceResult) DowngradeNonCanary() bool            { return a.downgradeNonCanary }
func (a AllocPlaceResult) MinJobVersion() uint64               { return a.minJobVersion }
func (a AllocPlaceResult) PreviousLost() bool                  { return a.lost }
func (a *AllocPlaceResult) SetTaskGroup(tg *structs.TaskGroup) { a.taskGroup = tg }

// allocDestructiveResult contains the information required to do a destructive
// update. Destructive changes should be applied atomically, as in the old alloc
// is only stopped if the new one can be placed.
type allocDestructiveResult struct {
	placeName             string
	placeTaskGroup        *structs.TaskGroup
	stopAlloc             *structs.Allocation
	stopStatusDescription string
}

func (a allocDestructiveResult) TaskGroup() *structs.TaskGroup                   { return a.placeTaskGroup }
func (a allocDestructiveResult) Name() string                                    { return a.placeName }
func (a allocDestructiveResult) Canary() bool                                    { return false }
func (a allocDestructiveResult) PreviousAllocation() *structs.Allocation         { return a.stopAlloc }
func (a allocDestructiveResult) SetPreviousAllocation(alloc *structs.Allocation) {} // NOOP
func (a allocDestructiveResult) IsRescheduling() bool                            { return false }
func (a allocDestructiveResult) StopPreviousAlloc() (bool, string) {
	return true, a.stopStatusDescription
}
func (a allocDestructiveResult) DowngradeNonCanary() bool { return false }
func (a allocDestructiveResult) MinJobVersion() uint64    { return 0 }
func (a allocDestructiveResult) PreviousLost() bool       { return false }

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

// filterOutByClientStatus returns all allocs from the set without the specified client status.
func (a allocSet) filterOutByClientStatus(clientStatuses ...string) allocSet {
	allocs := make(allocSet)
	for _, alloc := range a {
		if !slices.Contains(clientStatuses, alloc.ClientStatus) {
			allocs[alloc.ID] = alloc
		}
	}

	return allocs
}

// filterByClientStatus returns allocs from the set with the specified client status.
func (a allocSet) filterByClientStatus(clientStatus string) allocSet {
	allocs := make(allocSet)
	for _, alloc := range a {
		if alloc.ClientStatus == clientStatus {
			allocs[alloc.ID] = alloc
		}
	}

	return allocs
}

// AllocNameIndex is used to select allocation names for placement or removal
// given an existing set of placed allocations.
type AllocNameIndex struct {
	job, taskGroup string
	count          int
	b              structs.Bitmap

	// duplicates is used to store duplicate allocation indexes which are
	// currently present within the index tracking. The map key is the index,
	// and the current count of duplicates. The map is only accessed within a
	// single routine and multiple times per job scheduler invocation,
	// therefore no lock is used.
	duplicates map[uint]int
}

// newAllocNameIndex returns an allocNameIndex for use in selecting names of
// allocations to create or stop. It takes the job and task group name, desired
// count and any existing allocations as input.
func newAllocNameIndex(job, taskGroup string, count int, in allocSet) *AllocNameIndex {

	bitMap, duplicates := bitmapFrom(in, uint(count))

	return &AllocNameIndex{
		count:      count,
		b:          bitMap,
		job:        job,
		taskGroup:  taskGroup,
		duplicates: duplicates,
	}
}

// bitmapFrom creates a bitmap from the given allocation set and a minimum size
// maybe given. The size of the bitmap is as the larger of the passed minimum
// and the maximum alloc index of the passed input (byte aligned).
func bitmapFrom(input allocSet, minSize uint) (structs.Bitmap, map[uint]int) {
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

	// Initialize our duplicates mapping, allowing us to store a non-nil map
	// at the cost of 48 bytes.
	duplicates := make(map[uint]int)

	// Iterate through the allocSet input and hydrate the bitmap. We check that
	// the bitmap does not contain the index first, so we can track duplicate
	// entries.
	for _, a := range input {

		allocIndex := a.Index()

		if bitmap.Check(allocIndex) {
			duplicates[allocIndex]++
		} else {
			bitmap.Set(allocIndex)
		}
	}

	return bitmap, duplicates
}

// Highest removes and returns the highest n used names. The returned set
// can be less than n if there aren't n names set in the index
func (a *AllocNameIndex) Highest(n uint) map[string]struct{} {
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

// IsDuplicate checks whether the passed allocation index is duplicated within
// the tracking.
func (a *AllocNameIndex) IsDuplicate(idx uint) bool {
	val, ok := a.duplicates[idx]
	return ok && val > 0
}

// UnsetIndex unsets the index as having its name used
func (a *AllocNameIndex) UnsetIndex(idx uint) {

	// If this index is a duplicate, remove the duplicate entry. Otherwise, we
	// can remove it from the bitmap tracking.
	if num, ok := a.duplicates[idx]; ok {
		if num--; num == 0 {
			delete(a.duplicates, idx)
		}
	} else {
		a.b.Unset(idx)
	}
}

// NextCanaries returns the next n names for use as canaries and sets them as
// used. The existing canaries and destructive updates are also passed in.
func (a *AllocNameIndex) NextCanaries(n uint, existing, destructive allocSet) []string {
	next := make([]string, 0, n)

	// Create a name index
	existingNames := existing.nameSet()

	// First select indexes from the allocations that are undergoing
	// destructive updates. This way we avoid duplicate names as they will get
	// replaced. As this process already takes into account duplicate checking,
	// we can discard the duplicate mapping when generating the bitmap.
	dmap, _ := bitmapFrom(destructive, uint(a.count))
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
func (a *AllocNameIndex) Next(n uint) []string {
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
