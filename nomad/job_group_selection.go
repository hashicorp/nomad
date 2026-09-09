// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"math"
	"sort"

	"github.com/hashicorp/nomad/nomad/structs"
)

// maximumDesiredAllocations bounds steady allocation demand across every valid
// selection. Using the current assignments would let a later profile change
// exceed job_max_count without registering or scaling the job.
func maximumDesiredAllocations(job *structs.Job) int {
	total := 0
	add := func(count int) {
		if count > 0 {
			if count > math.MaxInt-total {
				total = math.MaxInt
			} else {
				total += count
			}
		}
	}
	for _, group := range job.TaskGroups {
		if job.TaskGroupSelection(group.Name) == nil {
			add(group.Count)
		}
	}
	for _, selection := range job.GroupSelections {
		if selection == nil || selection.Count <= 0 {
			continue
		}
		counts := make([]int, 0, len(selection.Groups))
		for _, name := range selection.Groups {
			if group := job.LookupTaskGroup(name); group != nil && group.Count > 0 {
				counts = append(counts, group.Count)
			}
		}
		sort.Sort(sort.Reverse(sort.IntSlice(counts)))
		for _, count := range counts[:min(selection.Count, len(counts))] {
			add(count)
		}
	}
	return total
}
