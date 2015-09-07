package scheduler

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

// allocTuple is a tuple of the allocation name and potential alloc ID
type allocTuple struct {
	Name      string
	TaskGroup *structs.TaskGroup
	Alloc     *structs.Allocation
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

// diffResult is used to return the sets that result from the diff
type diffResult struct {
	place, update, migrate, stop, ignore []allocTuple
}

func (d *diffResult) GoString() string {
	return fmt.Sprintf("allocs: (place %d) (update %d) (migrate %d) (stop %d) (ignore %d)",
		len(d.place), len(d.update), len(d.migrate), len(d.stop), len(d.ignore))
}

// diffAllocs is used to do a set difference between the target allocations
// and the existing allocations. This returns 5 sets of results, the list of
// named task groups that need to be placed (no existing allocation), the
// allocations that need to be updated (job definition is newer), allocs that
// need to be migrated (node is draining), the allocs that need to be evicted
// (no longer required), and those that should be ignored.
func diffAllocs(job *structs.Job, taintedNodes map[string]bool,
	required map[string]*structs.TaskGroup, allocs []*structs.Allocation) *diffResult {
	result := &diffResult{}

	// Scan the existing updates
	existing := make(map[string]struct{})
	for _, exist := range allocs {
		// Index the existing node
		name := exist.Name
		existing[name] = struct{}{}

		// Check for the definition in the required set
		tg, ok := required[name]

		// If not required, we stop the alloc
		if !ok {
			result.stop = append(result.stop, allocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     exist,
			})
			continue
		}

		// If we are on a tainted node, we must migrate
		if taintedNodes[exist.NodeID] {
			result.migrate = append(result.migrate, allocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     exist,
			})
			continue
		}

		// If the definition is updated we need to update
		// XXX: This is an extremely conservative approach. We can check
		// if the job definition has changed in a way that affects
		// this allocation and potentially ignore it.
		if job.ModifyIndex != exist.Job.ModifyIndex {
			result.update = append(result.update, allocTuple{
				Name:      name,
				TaskGroup: tg,
				Alloc:     exist,
			})
			continue
		}

		// Everything is up-to-date
		result.ignore = append(result.ignore, allocTuple{
			Name:      name,
			TaskGroup: tg,
			Alloc:     exist,
		})
	}

	// Scan the required groups
	for name, tg := range required {
		// Check for an existing allocation
		_, ok := existing[name]

		// Require a placement if no existing allocation. If there
		// is an existing allocation, we would have checked for a potential
		// update or ignore above.
		if !ok {
			result.place = append(result.place, allocTuple{
				Name:      name,
				TaskGroup: tg,
			})
		}
	}
	return result
}

// readyNodesInDCs returns all the ready nodes in the given datacenters
func readyNodesInDCs(state State, dcs []string) ([]*structs.Node, error) {
	// Index the DCs
	dcMap := make(map[string]struct{}, len(dcs))
	for _, dc := range dcs {
		dcMap[dc] = struct{}{}
	}

	// Scan the nodes
	var out []*structs.Node
	iter, err := state.Nodes()
	if err != nil {
		return nil, err
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		// Filter on datacenter and status
		node := raw.(*structs.Node)
		if node.Status != structs.NodeStatusReady {
			continue
		}
		if node.Drain {
			continue
		}
		if _, ok := dcMap[node.Datacenter]; !ok {
			continue
		}
		out = append(out, node)
	}
	return out, nil
}

// retryMax is used to retry a callback until it returns success or
// a maximum number of attempts is reached
func retryMax(max int, cb func() (bool, error)) error {
	attempts := 0
	for attempts < max {
		done, err := cb()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		attempts += 1
	}
	return &SetStatusError{
		Err:        fmt.Errorf("maximum attempts reached (%d)", max),
		EvalStatus: structs.EvalStatusFailed,
	}
}

// taintedNodes is used to scan the allocations and then check if the
// underlying nodes are tainted, and should force a migration of the allocation.
func taintedNodes(state State, allocs []*structs.Allocation) (map[string]bool, error) {
	out := make(map[string]bool)
	for _, alloc := range allocs {
		if _, ok := out[alloc.NodeID]; ok {
			continue
		}

		node, err := state.GetNodeByID(alloc.NodeID)
		if err != nil {
			return nil, err
		}

		// If the node does not exist, we should migrate
		if node == nil {
			out[alloc.NodeID] = true
			continue
		}

		out[alloc.NodeID] = structs.ShouldDrainNode(node.Status) || node.Drain
	}
	return out, nil
}
