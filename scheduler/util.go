package scheduler

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

// allocNameID is a tuple of the allocation name and potential alloc ID
type allocNameID struct {
	Name string
	ID   string
}

// materializeTaskGroups is used to materialize all the task groups
// a job requires. This is used to do the count expansion.
func materializeTaskGroups(job *structs.Job) map[string]*structs.TaskGroup {
	out := make(map[string]*structs.TaskGroup)
	for _, tg := range job.TaskGroups {
		for i := 0; i < tg.Count; i++ {
			name := fmt.Sprintf("%s.%s[%d]", job.Name, tg.Name, i)
			out[name] = tg
		}
	}
	return out
}

// indexAllocs is used to index a list of allocations by name
func indexAllocs(allocs []*structs.Allocation) map[string][]*structs.Allocation {
	out := make(map[string][]*structs.Allocation)
	for _, alloc := range allocs {
		name := alloc.Name
		out[name] = append(out[name], alloc)
	}
	return out
}

// diffAllocs is used to do a set difference between the target allocations
// and the existing allocations. This returns 5 sets of results, the list of
// named task groups that need to be placed (no existing allocation), the
// allocations that need to be updated (job definition is newer), allocs that
// need to be migrated (node is draining), the allocs that need to be evicted
// (no longer required), and those that should be ignored.
func diffAllocs(job *structs.Job,
	taintedNodes map[string]bool,
	required map[string]*structs.TaskGroup,
	existing map[string][]*structs.Allocation) (place, update, migrate, evict, ignore []allocNameID) {
	// Scan the existing updates
	for name, existList := range existing {
		for _, exist := range existList {
			// Check for the definition in the required set
			_, ok := required[name]

			// If not required, we evict
			if !ok {
				evict = append(evict, allocNameID{name, exist.ID})
				continue
			}

			// If we are on a tainted node, we must migrate
			if taintedNodes[exist.NodeID] {
				migrate = append(migrate, allocNameID{name, exist.ID})
				continue
			}

			// If the definition is updated we need to update
			// XXX: This is an extremely conservative approach. We can check
			// if the job definition has changed in a way that affects
			// this allocation and potentially ignore it.
			if job.ModifyIndex != exist.Job.ModifyIndex {
				update = append(update, allocNameID{name, exist.ID})
				continue
			}

			// Everything is up-to-date
			ignore = append(ignore, allocNameID{name, exist.ID})
		}
	}

	// Scan the required groups
	for name := range required {
		// Check for an existing allocation
		_, ok := existing[name]

		// Require a placement if no existing allocation. If there
		// is an existing allocation, we would have checked for a potential
		// update or ignore above.
		if !ok {
			place = append(place, allocNameID{name, ""})
		}
	}
	return
}

// addEvictsToPlan is used to add all the evictions to the plan
func addEvictsToPlan(plan *structs.Plan,
	evicts []allocNameID, indexed map[string][]*structs.Allocation) {
	for _, evict := range evicts {
		list := indexed[evict.Name]
		for _, alloc := range list {
			if alloc.ID == evict.ID {
				plan.AppendEvict(alloc)
			}
		}
	}
}
