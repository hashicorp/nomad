// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

/*

	Comparables are flattened resources from allocations
	and nodes that are able to be compared with eachother

	This is used mainly to check if allocations fit on
	nodes, and for preemption.

	Some of the structs and methods in this file are fairly
	shallow, and that should be fixed. However users of
	these structs will know exactly what is being passed around
	as opposed to the previous ComparableResources which held
	everything but were often sparsely populated and consumed
	vast resources.

*/

// BaseComparableResource is the base set of resources used to compare
// allocations for node fitment and preemption.
//
// When using this, you should have already determined (or should
// soon determine) fitment for networking and devices.
type BaseComparableResource struct {
	ComparableCPU
	ComparableMem
	ComparableDisk
}

func (c *BaseComparableResource) Add(other *BaseComparableResource) {
	if other == nil {
		return
	}
	c.ComparableCPU.Add(&other.ComparableCPU)
	c.ComparableMem.Add(&other.ComparableMem)
	c.ComparableDisk.Add(&other.ComparableDisk)
}

func (c *BaseComparableResource) Subtract(other *BaseComparableResource) {
	if other == nil {
		return
	}
	c.ComparableCPU.Subtract(&other.ComparableCPU)
	c.ComparableMem.Subtract(&other.ComparableMem)
	c.ComparableDisk.Subtract(&other.ComparableDisk)
}

func (c *BaseComparableResource) Superset(other *BaseComparableResource) (bool, string) {
	if !c.ComparableCPU.Superset(&other.ComparableCPU) {
		return false, "cpu"
	}
	if !c.ComparableMem.Superset(&other.ComparableMem) {
		return false, "mem"
	}
	if !c.ComparableDisk.Superset(&other.ComparableDisk) {
		return false, "disk"
	}
	return true, ""
}

func (c *BaseComparableResource) Copy() *BaseComparableResource {
	return &BaseComparableResource{
		ComparableCPU:  c.ComparableCPU,
		ComparableMem:  c.ComparableMem,
		ComparableDisk: c.ComparableDisk,
	}
}

// TODO: Remove the indirection from these shallow struct.
// but it will require moving the code for Add(). We will save that
// as a followup
type ComparableCPU struct {
	AllocatedCpuResources
}

func (c *ComparableCPU) Add(delta *ComparableCPU) {
	if delta == nil {
		return
	}

	c.AllocatedCpuResources.Add(&delta.AllocatedCpuResources)
}

func (c *ComparableCPU) Subtract(delta *ComparableCPU) {
	if delta == nil {
		return
	}

	c.AllocatedCpuResources.Subtract(&delta.AllocatedCpuResources)
}

func (c *ComparableCPU) Superset(other *ComparableCPU) bool {
	if c.CpuShares < other.CpuShares {
		return false
	}

	if len(c.ReservedCores) == 0 {
		return true
	}

	tmp := make(map[uint16]struct{}, len(c.ReservedCores))

	for _, v := range c.ReservedCores {
		tmp[v] = struct{}{}
	}
	for _, v := range other.ReservedCores {
		if _, ok := tmp[v]; !ok {
			return false
		}
	}

	return true
}

// Future work: Remove the indirection from this shallow struct.
// but it will require moving the code for Add(). We will save that
// as a followup
type ComparableMem struct {
	AllocatedMemoryResources
}

func (c *ComparableMem) Add(delta *ComparableMem) {
	if delta == nil {
		return
	}

	c.AllocatedMemoryResources.Add(&delta.AllocatedMemoryResources)
}

func (c *ComparableMem) Subtract(delta *ComparableMem) {
	if delta == nil {
		return
	}

	c.AllocatedMemoryResources.Subtract(&delta.AllocatedMemoryResources)
}

func (c *ComparableMem) Superset(other *ComparableMem) bool {
	if other == nil {
		return false
	}
	return c.MemoryMB >= other.MemoryMB
}

type ComparableDisk struct {
	DiskMB int64
}

func (c *ComparableDisk) Add(delta *ComparableDisk) {
	if delta == nil {
		return
	}

	c.DiskMB += delta.DiskMB
}

func (c *ComparableDisk) Subtract(delta *ComparableDisk) {
	if delta == nil {
		return
	}

	c.DiskMB -= delta.DiskMB
}

func (c *ComparableDisk) Superset(other *ComparableDisk) bool {
	if other == nil {
		return false
	}
	return c.DiskMB >= other.DiskMB
}

type ComparableNetworks struct {
	FlattenedNetworks Networks
	SharedNetworks    Networks
	SharedPorts       AllocatedPorts
}

func (c *ComparableNetworks) Add(delta *ComparableNetworks) {
	if delta == nil {
		return
	}
	c.FlattenedNetworks.Add(&delta.FlattenedNetworks)
	c.SharedNetworks.Add(&delta.SharedNetworks)
	// Skip adding ports to have maintain previous Comparable behavior
}

func (c *ComparableNetworks) Copy() *ComparableNetworks {
	n := new(ComparableNetworks)

	n.FlattenedNetworks = c.FlattenedNetworks.Copy()
	n.SharedNetworks = c.SharedNetworks.Copy()
	n.SharedPorts = append(n.SharedPorts, c.SharedPorts...)

	return n
}
