package client

import (
	"fmt"
	"math/rand"

	"github.com/hashicorp/nomad/nomad/structs"
)

type allocTuple struct {
	exist, updated *structs.Allocation
}

// diffResult is used to return the sets that result from a diff
type diffResult struct {
	added   []*structs.Allocation
	removed []*structs.Allocation
	updated []allocTuple
	ignore  []*structs.Allocation
}

func (d *diffResult) GoString() string {
	return fmt.Sprintf("allocs: (added %d) (removed %d) (updated %d) (ignore %d)",
		len(d.added), len(d.removed), len(d.updated), len(d.ignore))
}

// diffAllocs is used to diff the existing and updated allocations
// to see what has happened.
func diffAllocs(existing []*structs.Allocation, allocs *allocUpdates) *diffResult {
	// Scan the existing allocations
	result := &diffResult{}
	existIdx := make(map[string]struct{})
	for _, exist := range existing {
		// Mark this as existing
		existIdx[exist.ID] = struct{}{}

		// Check if the alloc was updated or filtered because an update wasn't
		// needed.
		alloc, pulled := allocs.pulled[exist.ID]
		_, filtered := allocs.filtered[exist.ID]

		// If not updated or filtered, removed
		if !pulled && !filtered {
			result.removed = append(result.removed, exist)
			continue
		}

		// Check for an update
		if pulled && alloc.AllocModifyIndex > exist.AllocModifyIndex {
			result.updated = append(result.updated, allocTuple{exist, alloc})
			continue
		}

		// Ignore this
		result.ignore = append(result.ignore, exist)
	}

	// Scan the updated allocations for any that are new
	for id, pulled := range allocs.pulled {
		if _, ok := existIdx[id]; !ok {
			result.added = append(result.added, pulled)
		}
	}
	return result
}

// shuffleStrings randomly shuffles the list of strings
func shuffleStrings(list []string) {
	for i := range list {
		j := rand.Intn(i + 1)
		list[i], list[j] = list[j], list[i]
	}
}
