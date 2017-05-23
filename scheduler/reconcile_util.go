package scheduler

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

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
	for _, tg := range job.TaskGroups {
		s, ok := m[tg.Name]
		if !ok {
			s = make(map[string]*structs.Allocation)
			m[tg.Name] = s
		}
	}
	return m
}

// allocSet is a set of allocations with a series of helper functions defined
// that help reconcile state.
type allocSet map[string]*structs.Allocation

// newAllocSet creates an allocation set given a set of allocations
func newAllocSet(allocs []*structs.Allocation) allocSet {
	s := make(map[string]*structs.Allocation, len(allocs))
	for _, a := range allocs {
		s[a.ID] = a
	}
	return s
}

// GoString provides a human readable view of the set
func (a allocSet) GoString() string {
	if len(a) == 0 {
		return "[]"
	}

	start := fmt.Sprintf("len(%d) [\n", len(a))
	for k := range a {
		start += k + ",\n"
	}
	return start + "]"
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

// fitlerByTainted takes a set of tainted nodes and filters the allocation set
// into three groups:
// 1. Those that exist on untainted nodes
// 2. Those exist on nodes that are draining
// 3. Those that exist on lost nodes
func (a allocSet) filterByTainted(nodes map[string]*structs.Node) (untainted, migrate, lost allocSet) {
	untainted = make(map[string]*structs.Allocation)
	migrate = make(map[string]*structs.Allocation)
	lost = make(map[string]*structs.Allocation)
	for _, alloc := range a {
		n, ok := nodes[alloc.NodeID]
		switch {
		case !ok:
			untainted[alloc.ID] = alloc
		case n == nil || n.TerminalStatus():
			lost[alloc.ID] = alloc
		default:
			migrate[alloc.ID] = alloc
		}
	}
	return
}

// filterByCanary returns a new allocation set that contains only canaries
func (a allocSet) filterByCanary() allocSet {
	canaries := make(map[string]*structs.Allocation)
	for _, alloc := range a {
		if alloc.Canary {
			canaries[alloc.ID] = alloc
		}
	}
	return canaries
}

// filterByDeployment filters allocations into two sets, those that match the
// given deployment ID and those that don't
func (a allocSet) filterByDeployment(id string) (match, nonmatch allocSet) {
	match = make(map[string]*structs.Allocation)
	nonmatch = make(map[string]*structs.Allocation)
	for _, alloc := range a {
		if alloc.DeploymentID == id {
			match[alloc.ID] = alloc
		} else {
			nonmatch[alloc.ID] = alloc
		}
	}
	return
}
