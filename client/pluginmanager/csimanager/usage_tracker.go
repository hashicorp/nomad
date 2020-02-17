package csimanager

import (
	"sync"

	"github.com/hashicorp/nomad/nomad/structs"
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
	volume    *structs.CSIVolume
	usageOpts UsageOptions
}

func (v *volumeUsageTracker) allocsForKey(key volumeUsageKey) []string {
	return v.state[key]
}

func (v *volumeUsageTracker) appendAlloc(key volumeUsageKey, alloc *structs.Allocation) {
	allocs := v.allocsForKey(key)
	allocs = append(allocs, alloc.ID)
	v.state[key] = allocs
}

func (v *volumeUsageTracker) removeAlloc(key volumeUsageKey, needle *structs.Allocation) {
	allocs := v.allocsForKey(key)
	var newAllocs []string
	for _, allocID := range allocs {
		if allocID != needle.ID {
			newAllocs = append(newAllocs, allocID)
		}
	}

	if len(newAllocs) == 0 {
		delete(v.state, key)
	} else {
		v.state[key] = newAllocs
	}
}

func (v *volumeUsageTracker) Claim(alloc *structs.Allocation, volume *structs.CSIVolume, usage *UsageOptions) {
	v.stateMu.Lock()
	defer v.stateMu.Unlock()

	key := volumeUsageKey{volume: volume, usageOpts: *usage}
	v.appendAlloc(key, alloc)
}

// Free removes the allocation from the state list for the given alloc. If the
// alloc is the last allocation for the volume then it returns true.
func (v *volumeUsageTracker) Free(alloc *structs.Allocation, volume *structs.CSIVolume, usage *UsageOptions) bool {
	v.stateMu.Lock()
	defer v.stateMu.Unlock()

	key := volumeUsageKey{volume: volume, usageOpts: *usage}
	v.removeAlloc(key, alloc)
	allocs := v.allocsForKey(key)
	return len(allocs) == 0
}
