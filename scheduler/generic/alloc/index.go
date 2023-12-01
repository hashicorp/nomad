// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package alloc

import "github.com/hashicorp/nomad/nomad/structs"

// NameIndex is used to select allocation names for placement or removal
// given an existing set of placed allocations.
type NameIndex struct {
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

// NewNameIndex returns an allocNameIndex for use in selecting names of
// allocations to create or stop. It takes the job and task group name, desired
// count and any existing allocations as input.
func NewNameIndex(job, taskGroup string, count int, in Set) *NameIndex {

	bitMap, duplicates := bitmapFrom(in, uint(count))

	return &NameIndex{
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
func bitmapFrom(input Set, minSize uint) (structs.Bitmap, map[uint]int) {
	var maxSize uint
	for _, a := range input {
		if num := a.Index(); num > maxSize {
			maxSize = num
		}
	}

	if l := uint(len(input)); minSize < l {
		minSize = l
	}

	if maxSize < minSize {
		maxSize = minSize
	} else if maxSize%8 == 0 {
		// This may be possible if the job was scaled down. We want to make sure
		// that the maxSize index is not byte-aligned otherwise we will overflow
		// the bitmap.
		maxSize++
	}

	if maxSize == 0 {
		maxSize = 8
	}

	// byteAlign the count
	if remainder := maxSize % 8; remainder != 0 {
		maxSize = maxSize + 8 - remainder
	}

	bitmap, err := structs.NewBitmap(maxSize)
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
func (a *NameIndex) Highest(n uint) map[string]struct{} {
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
func (a *NameIndex) IsDuplicate(idx uint) bool {
	val, ok := a.duplicates[idx]
	return ok && val > 0
}

// UnsetIndex unsets the index as having its name used
func (a *NameIndex) UnsetIndex(idx uint) {

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
func (a *NameIndex) NextCanaries(n uint, existing, destructive Set) []string {
	next := make([]string, 0, n)

	// Create a name index
	existingNames := existing.NameSet()

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
func (a *NameIndex) Next(n uint) []string {
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
