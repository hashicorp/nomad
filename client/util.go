package client

import (
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"time"

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
func diffAllocs(existing, updated []*structs.Allocation) *diffResult {
	result := &diffResult{}

	// Index the updated allocations by id
	idx := make(map[string]*structs.Allocation)
	for _, update := range updated {
		idx[update.ID] = update
	}

	// Scan the existing allocations
	existIdx := make(map[string]struct{})
	for _, exist := range existing {
		// Mark this as existing
		existIdx[exist.ID] = struct{}{}

		// Check for presence in the new set
		update, ok := idx[exist.ID]

		// If not present, removed
		if !ok {
			result.removed = append(result.removed, exist)
			continue
		}

		// Check for an update
		if update.ModifyIndex > exist.ModifyIndex {
			result.updated = append(result.updated, allocTuple{exist, update})
			continue
		}

		// Ignore this
		result.ignore = append(result.ignore, exist)
	}

	// Scan the updated allocations for any that are new
	for _, update := range updated {
		if _, ok := existIdx[update.ID]; !ok {
			result.added = append(result.added, update)
		}
	}
	return result
}

// generateUUID is used to generate a random UUID
func generateUUID() string {
	buf := make([]byte, 16)
	if _, err := crand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}

// Returns a random stagger interval between 0 and the duration
func randomStagger(intv time.Duration) time.Duration {
	return time.Duration(uint64(rand.Int63()) % uint64(intv))
}

// shuffleStrings randomly shuffles the list of strings
func shuffleStrings(list []string) {
	for i := range list {
		j := rand.Intn(i + 1)
		list[i], list[j] = list[j], list[i]
	}
}
