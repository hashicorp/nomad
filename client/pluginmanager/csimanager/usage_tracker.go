// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package csimanager

import (
	"sync"
)

// volumeUsageTracker tracks the allocations that depend on a given volume
type volumeUsageTracker struct {
	// state is a map of volumeUsageKey to a slice of allocation ids
	state   map[volumeUsageKey][]string
	stateMu sync.Mutex
}

func newVolumeUsageTracker() *volumeUsageTracker {
	return &volumeUsageTracker{
		state: make(map[volumeUsageKey][]string),
	}
}

type volumeUsageKey struct {
	id        string
	usageOpts UsageOptions
}

func (v *volumeUsageTracker) allocsForKey(key volumeUsageKey) []string {
	return v.state[key]
}

func (v *volumeUsageTracker) appendAlloc(key volumeUsageKey, allocID string) {
	allocs := v.allocsForKey(key)
	allocs = append(allocs, allocID)
	v.state[key] = allocs
}

func (v *volumeUsageTracker) removeAlloc(key volumeUsageKey, needle string) {
	allocs := v.allocsForKey(key)
	var newAllocs []string
	for _, allocID := range allocs {
		if allocID != needle {
			newAllocs = append(newAllocs, allocID)
		}
	}

	if len(newAllocs) == 0 {
		delete(v.state, key)
	} else {
		v.state[key] = newAllocs
	}
}

func (v *volumeUsageTracker) Claim(allocID, volID string, usage *UsageOptions) {
	v.stateMu.Lock()
	defer v.stateMu.Unlock()

	key := volumeUsageKey{id: volID, usageOpts: *usage}
	v.appendAlloc(key, allocID)
}

// Free removes the allocation from the state list for the given alloc. If the
// alloc is the last allocation for the volume then it returns true.
func (v *volumeUsageTracker) Free(allocID, volID string, usage *UsageOptions) bool {
	v.stateMu.Lock()
	defer v.stateMu.Unlock()

	key := volumeUsageKey{id: volID, usageOpts: *usage}
	v.removeAlloc(key, allocID)
	allocs := v.allocsForKey(key)
	return len(allocs) == 0
}
