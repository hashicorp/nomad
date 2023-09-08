// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// diffResult is used to return the sets that result from a diff
type diffResult struct {
	added   []*structs.Allocation
	removed []string
	updated []*structs.Allocation
	ignore  []string
}

func (d *diffResult) GoString() string {
	return fmt.Sprintf("allocs: (added %d) (removed %d) (updated %d) (ignore %d)",
		len(d.added), len(d.removed), len(d.updated), len(d.ignore))
}

// diffAllocs is used to diff the existing and updated allocations
// to see what has happened.
func diffAllocs(existing map[string]uint64, allocs *allocUpdates) *diffResult {
	// Scan the existing allocations
	result := &diffResult{}
	for existID, existIndex := range existing {
		// Check if the alloc was updated or filtered because an update wasn't
		// needed.
		alloc, pulled := allocs.pulled[existID]
		_, filtered := allocs.filtered[existID]

		// If not updated or filtered, removed
		if !pulled && !filtered {
			result.removed = append(result.removed, existID)
			continue
		}

		// Check for an update (note: AllocModifyIndex is only updated for
		// server updates)
		if pulled && alloc.AllocModifyIndex > existIndex {
			result.updated = append(result.updated, alloc)
			continue
		}

		// Ignore this
		result.ignore = append(result.ignore, existID)
	}

	// Scan the updated allocations for any that are new
	for id, pulled := range allocs.pulled {
		if _, ok := existing[id]; !ok {
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

// stoppedTimer returns a timer that's stopped and wouldn't fire until
// it's reset
func stoppedTimer() *time.Timer {
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	return timer
}
