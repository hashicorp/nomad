// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package alloc

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// Test that we properly create the bitmap even when the alloc set includes an
// allocation with a higher count than the current min count and it is byte
// aligned.
// Ensure no regression from: https://github.com/hashicorp/nomad/issues/3008
func TestBitmapFrom(t *testing.T) {
	ci.Parallel(t)

	input := map[string]*structs.Allocation{
		"8": {
			JobID:     "foo",
			TaskGroup: "bar",
			Name:      "foo.bar[8]",
		},
	}
	b, dups := bitmapFrom(input, 1)
	must.Eq(t, 16, b.Size())
	must.MapEmpty(t, dups)

	b, dups = bitmapFrom(input, 8)
	must.Eq(t, 16, b.Size())
	must.MapEmpty(t, dups)
}

func Test_allocNameIndex_Highest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputAllocNameIndex *NameIndex
		inputN              uint
		expectedOutput      map[string]struct{}
	}{
		{
			name: "select 1",
			inputAllocNameIndex: NewNameIndex(
				"example", "cache", 3, map[string]*structs.Allocation{
					"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
						Name:      "example.cache[0]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"e24771e6-8900-5d2d-ec93-e7076284774a": {
						Name:      "example.cache[1]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
						Name:      "example.cache[2]",
						JobID:     "example",
						TaskGroup: "cache",
					},
				}),
			inputN: 1,
			expectedOutput: map[string]struct{}{
				"example.cache[2]": {},
			},
		},
		{
			name: "select all",
			inputAllocNameIndex: NewNameIndex(
				"example", "cache", 3, map[string]*structs.Allocation{
					"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
						Name:      "example.cache[0]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"e24771e6-8900-5d2d-ec93-e7076284774a": {
						Name:      "example.cache[1]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
						Name:      "example.cache[2]",
						JobID:     "example",
						TaskGroup: "cache",
					},
				}),
			inputN: 3,
			expectedOutput: map[string]struct{}{
				"example.cache[2]": {},
				"example.cache[1]": {},
				"example.cache[0]": {},
			},
		},
		{
			name: "select too many",
			inputAllocNameIndex: NewNameIndex(
				"example", "cache", 3, map[string]*structs.Allocation{
					"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
						Name:      "example.cache[0]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"e24771e6-8900-5d2d-ec93-e7076284774a": {
						Name:      "example.cache[1]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
						Name:      "example.cache[2]",
						JobID:     "example",
						TaskGroup: "cache",
					},
				}),
			inputN: 13,
			expectedOutput: map[string]struct{}{
				"example.cache[2]": {},
				"example.cache[1]": {},
				"example.cache[0]": {},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expectedOutput, tc.inputAllocNameIndex.Highest(tc.inputN))
		})
	}
}

func Test_allocNameIndex_NextCanaries(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputAllocNameIndex *NameIndex
		inputN              uint
		inputExisting       Set
		inputDestructive    Set
		expectedOutput      []string
	}{
		{
			name: "single canary",
			inputAllocNameIndex: NewNameIndex(
				"example", "cache", 3, map[string]*structs.Allocation{
					"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
						Name:      "example.cache[0]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"e24771e6-8900-5d2d-ec93-e7076284774a": {
						Name:      "example.cache[1]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
						Name:      "example.cache[2]",
						JobID:     "example",
						TaskGroup: "cache",
					},
				}),
			inputN:        1,
			inputExisting: nil,
			inputDestructive: map[string]*structs.Allocation{
				"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
					Name:      "example.cache[0]",
					JobID:     "example",
					TaskGroup: "cache",
				},
				"e24771e6-8900-5d2d-ec93-e7076284774a": {
					Name:      "example.cache[1]",
					JobID:     "example",
					TaskGroup: "cache",
				},
				"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
					Name:      "example.cache[2]",
					JobID:     "example",
					TaskGroup: "cache",
				},
			},
			expectedOutput: []string{
				"example.cache[0]",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.SliceContainsAll(
				t, tc.expectedOutput,
				tc.inputAllocNameIndex.NextCanaries(tc.inputN, tc.inputExisting, tc.inputDestructive))
		})
	}
}

func Test_allocNameIndex_Next(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputAllocNameIndex *NameIndex
		inputN              uint
		expectedOutput      []string
	}{
		{
			name:                "empty existing bitmap",
			inputAllocNameIndex: NewNameIndex("example", "cache", 3, nil),
			inputN:              3,
			expectedOutput: []string{
				"example.cache[0]", "example.cache[1]", "example.cache[2]",
			},
		},
		{
			name: "non-empty existing bitmap simple",
			inputAllocNameIndex: NewNameIndex(
				"example", "cache", 3, map[string]*structs.Allocation{
					"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
						Name:      "example.cache[0]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"e24771e6-8900-5d2d-ec93-e7076284774a": {
						Name:      "example.cache[1]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
						Name:      "example.cache[2]",
						JobID:     "example",
						TaskGroup: "cache",
					},
				}),
			inputN: 3,
			expectedOutput: []string{
				"example.cache[0]", "example.cache[1]", "example.cache[2]",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.SliceContainsAll(t, tc.expectedOutput, tc.inputAllocNameIndex.Next(tc.inputN))
		})
	}
}

func Test_allocNameIndex_Duplicates(t *testing.T) {
	ci.Parallel(t)

	inputAllocSet := map[string]*structs.Allocation{
		"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
			Name:      "example.cache[0]",
			JobID:     "example",
			TaskGroup: "cache",
		},
		"e24771e6-8900-5d2d-ec93-e7076284774a": {
			Name:      "example.cache[1]",
			JobID:     "example",
			TaskGroup: "cache",
		},
		"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
			Name:      "example.cache[2]",
			JobID:     "example",
			TaskGroup: "cache",
		},
		"76a6a487-016b-2fc2-8295-d811473ca93d": {
			Name:      "example.cache[0]",
			JobID:     "example",
			TaskGroup: "cache",
		},
	}

	// Build the tracker, and check some key information.
	allocNameIndexTracker := NewNameIndex("example", "cache", 4, inputAllocSet)
	must.Eq(t, 8, allocNameIndexTracker.b.Size())
	must.MapLen(t, 1, allocNameIndexTracker.duplicates)
	must.True(t, allocNameIndexTracker.IsDuplicate(0))

	// Unsetting the index should remove the duplicate entry, but not the entry
	// from the underlying bitmap.
	allocNameIndexTracker.UnsetIndex(0)
	must.MapLen(t, 0, allocNameIndexTracker.duplicates)
	must.True(t, allocNameIndexTracker.b.Check(0))

	// If we now select a new index, having previously checked for a duplicate,
	// we should get a non-duplicate.
	nextAllocNames := allocNameIndexTracker.Next(1)
	must.Len(t, 1, nextAllocNames)
	must.Eq(t, "example.cache[3]", nextAllocNames[0])
}

func TestAllocSet_filterByRescheduleable(t *testing.T) {
	ci.Parallel(t)

	noRescheduleJob := mock.Job()
	noRescheduleTG := &structs.TaskGroup{
		Name: "noRescheduleTG",
		ReschedulePolicy: &structs.ReschedulePolicy{
			Attempts:  0,
			Unlimited: false,
		},
	}

	noRescheduleJob.TaskGroups[0] = noRescheduleTG

	testJob := mock.Job()
	rescheduleTG := &structs.TaskGroup{
		Name: "rescheduleTG",
		ReschedulePolicy: &structs.ReschedulePolicy{
			Attempts:  1,
			Unlimited: false,
		},
	}
	testJob.TaskGroups[0] = rescheduleTG

	now := time.Now()

	type testCase struct {
		name                        string
		all                         Set
		isBatch                     bool
		supportsDisconnectedClients bool
		isDisconnecting             bool
		deployment                  *structs.Deployment

		// expected results
		untainted Set
		resNow    Set
		resLater  []*DelayedRescheduleInfo
	}

	testCases := []testCase{
		{
			name:            "batch disconnecting allocation no reschedule",
			isDisconnecting: true,
			isBatch:         true,
			all: Set{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          noRescheduleJob,
					TaskGroup:    "noRescheduleTG",
				},
			},
			untainted: Set{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          noRescheduleJob,
					TaskGroup:    "noRescheduleTG",
				},
			},
			resNow:   Set{},
			resLater: []*DelayedRescheduleInfo{},
		},
		{
			name:            "batch ignore unknown disconnecting allocs",
			isDisconnecting: true,
			isBatch:         true,
			all: Set{
				"disconnecting1": {
					ID:           "disconnection1",
					ClientStatus: structs.AllocClientStatusUnknown,
					Job:          testJob,
				},
			},
			untainted: Set{},
			resNow:    Set{},
			resLater:  []*DelayedRescheduleInfo{},
		},
		{
			name:            "batch disconnecting allocation reschedule",
			isDisconnecting: true,
			isBatch:         true,
			all: Set{
				"rescheduleNow1": {
					ID:           "rescheduleNow1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					TaskGroup:    "rescheduleTG",
				},
			},
			untainted: Set{},
			resNow: Set{
				"rescheduleNow1": {
					ID:           "rescheduleNow1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					TaskGroup:    "rescheduleTG",
				},
			},
			resLater: []*DelayedRescheduleInfo{},
		},
		{
			name:            "service disconnecting allocation no reschedule",
			isDisconnecting: true,
			isBatch:         false,
			all: Set{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          noRescheduleJob,
					TaskGroup:    "noRescheduleTG",
				},
			},
			untainted: Set{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          noRescheduleJob,
					TaskGroup:    "noRescheduleTG",
				},
			},
			resNow:   Set{},
			resLater: []*DelayedRescheduleInfo{},
		},
		{
			name:            "service disconnecting allocation reschedule",
			isDisconnecting: true,
			isBatch:         false,
			all: Set{
				"rescheduleNow1": {
					ID:           "rescheduleNow1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					TaskGroup:    "rescheduleTG",
				},
			},
			untainted: Set{},
			resNow: Set{
				"rescheduleNow1": {
					ID:           "rescheduleNow1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					TaskGroup:    "rescheduleTG",
				},
			},
			resLater: []*DelayedRescheduleInfo{},
		},
		{
			name:            "service ignore unknown disconnecting allocs",
			isDisconnecting: true,
			isBatch:         false,
			all: Set{
				"disconnecting1": {
					ID:           "disconnection1",
					ClientStatus: structs.AllocClientStatusUnknown,
					Job:          testJob,
				},
			},
			untainted: Set{},
			resNow:    Set{},
			resLater:  []*DelayedRescheduleInfo{},
		},
		{
			name:            "service running allocation no reschedule",
			isDisconnecting: false,
			isBatch:         true,
			all: Set{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          noRescheduleJob,
					TaskGroup:    "noRescheduleTG",
				},
			},
			untainted: Set{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          noRescheduleJob,
					TaskGroup:    "noRescheduleTG",
				},
			},
			resNow:   Set{},
			resLater: []*DelayedRescheduleInfo{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			untainted, resNow, resLater := tc.all.FilterByRescheduleable(tc.isBatch,
				tc.isDisconnecting, now, "evailID", tc.deployment)
			must.Eq(t, tc.untainted, untainted, must.Sprintf("with-nodes: untainted"))
			must.Eq(t, tc.resNow, resNow, must.Sprintf("with-nodes: reschedule-now"))
			must.Eq(t, tc.resLater, resLater, must.Sprintf("with-nodes: rescheduleLater"))
		})
	}
}
