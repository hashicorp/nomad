package structs

// RemoveAllocs is used to remove any allocs with the given IDs
// from the list of allocations
func RemoveAllocs(alloc []*Allocation, remove []string) []*Allocation {
	// Convert remove into a set
	removeSet := make(map[string]struct{})
	for _, removeID := range remove {
		removeSet[removeID] = struct{}{}
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
func AllocsFit(node *Node, allocs []*Allocation) (bool, error) {
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
			return false, err
		}
	}

	// For each alloc, add the resources
	for _, alloc := range allocs {
		if err := used.Add(alloc.Resources); err != nil {
			return false, err
		}
	}

	// Check that the node resources are a super set of those
	// that are being allocated
	if !node.Resources.Superset(used) {
		return false, nil
	}

	// Ensure ports are not over commited
	if PortsOvercommited(used) {
		return false, nil
	}

	// Allocations fit!
	return true, nil
}
