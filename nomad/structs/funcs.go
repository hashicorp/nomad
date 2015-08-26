package structs

import "math"

// RemoveAllocs is used to remove any allocs with the given IDs
// from the list of allocations
func RemoveAllocs(alloc []*Allocation, remove []*Allocation) []*Allocation {
	// Convert remove into a set
	removeSet := make(map[string]struct{})
	for _, remove := range remove {
		removeSet[remove.ID] = struct{}{}
	}

	n := len(alloc)
	for i := 0; i < n; i++ {
		if _, ok := removeSet[alloc[i].ID]; ok {
			alloc[i], alloc[n-1] = alloc[n-1], nil
			i--
			n--
		}
	}

	alloc = alloc[:n]
	return alloc
}

// FilterTerminalAllocs filters out all allocations in a terminal state
func FilterTerminalAllocs(allocs []*Allocation) []*Allocation {
	n := len(allocs)
	for i := 0; i < n; i++ {
		if allocs[i].TerminalStatus() {
			allocs[i], allocs[n-1] = allocs[n-1], nil
			i--
			n--
		}
	}
	return allocs[:n]
}

// PortsOvercommited checks if any ports are over-committed.
// This does not handle CIDR subsets, and computes for the entire
// CIDR block currently.
func PortsOvercommited(r *Resources) bool {
	for _, net := range r.Networks {
		ports := make(map[int]struct{})
		for _, port := range net.ReservedPorts {
			if _, ok := ports[port]; ok {
				return true
			}
			ports[port] = struct{}{}
		}
	}
	return false
}

// AllocsFit checks if a given set of allocations will fit on a node
func AllocsFit(node *Node, allocs []*Allocation) (bool, *Resources, error) {
	// Compute the utilization from zero
	used := new(Resources)
	for _, net := range node.Resources.Networks {
		used.Networks = append(used.Networks, &NetworkResource{
			Public: net.Public,
			CIDR:   net.CIDR,
		})
	}

	// Add the reserved resources of the node
	if node.Reserved != nil {
		if err := used.Add(node.Reserved); err != nil {
			return false, nil, err
		}
	}

	// For each alloc, add the resources
	for _, alloc := range allocs {
		if err := used.Add(alloc.Resources); err != nil {
			return false, nil, err
		}
	}

	// Check that the node resources are a super set of those
	// that are being allocated
	if !node.Resources.Superset(used) {
		return false, used, nil
	}

	// Ensure ports are not over commited
	if PortsOvercommited(used) {
		return false, used, nil
	}

	// Allocations fit!
	return true, used, nil
}

// ScoreFit is used to score the fit based on the Google work published here:
// http://www.columbia.edu/~cs2035/courses/ieor4405.S13/datacenter_scheduling.ppt
// This is equivalent to their BestFit v3
func ScoreFit(node *Node, util *Resources) float64 {
	// Determine the node availability
	nodeCpu := node.Resources.CPU
	if node.Reserved != nil {
		nodeCpu -= node.Reserved.CPU
	}
	nodeMem := float64(node.Resources.MemoryMB)
	if node.Reserved != nil {
		nodeMem -= float64(node.Reserved.MemoryMB)
	}

	// Compute the free percentage
	freePctCpu := 1 - (util.CPU / nodeCpu)
	freePctRam := 1 - (float64(util.MemoryMB) / nodeMem)

	// Total will be "maximized" the smaller the value is.
	// At 100% utilization, the total is 2, while at 0% util it is 20.
	total := math.Pow(10, freePctCpu) + math.Pow(10, freePctRam)

	// Invert so that the "maximized" total represents a high-value
	// score. Because the floor is 20, we simply use that as an anchor.
	// This means at a perfect fit, we return 18 as the score.
	score := 20.0 - total

	// Bound the score, just in case
	// If the score is over 18, that means we've overfit the node.
	if score > 18.0 {
		score = 18.0
	} else if score < 0 {
		score = 0
	}
	return score
}
