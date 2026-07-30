package structs

import (
	"github.com/hashicorp/nomad/client/lib/idset"
)

/*

	Comparables are flattened resources from allocations
	and nodes that are able to be compared with eachother

	This is used mainly to check for allocation fitment on
	nodes, and for preemption.

	Some of the structs and methods in this file are fairly
	shallow, and that should be fixed. However users of
	these structs will know exactly what is being passed around
	as opposed to the previous ComparableResources which held
	everything but were often sparsely populated and consumed
	vast resources.

*/

// ComparableResourcesV2 are the base set of resources used to compare
// allocations for node fit and preemption.
//
// When using this, you should have already determined (or should
// soon determine) fitment for networking and devices.
type ComparableResourcesV2 struct {
	*ComparableCPU
	*ComparableMem
	*ComparableDisk
}

func (c *ComparableResourcesV2) Add(other *ComparableResourcesV2) {
	c.ComparableCPU.Add(other.ComparableCPU)
	c.ComparableMem.Add(other.ComparableMem)
	c.ComparableDisk.Add(other.ComparableDisk)
}

func (c *ComparableResourcesV2) Subtract(other *ComparableResourcesV2) {
	c.ComparableCPU.Subtract(other.ComparableCPU)
	c.ComparableMem.Subtract(other.ComparableMem)
	c.ComparableDisk.Subtract(other.ComparableDisk)
}

func (c *ComparableResourcesV2) Superset(other *ComparableResourcesV2) (bool, string) {
	if !c.ComparableCPU.Superset(other.ComparableCPU) {
		return false, "cpu"
	}
	if !c.ComparableMem.Superset(other.ComparableMem) {
		return false, "mem"
	}
	if !c.ComparableDisk.Superset(other.ComparableDisk) {
		return false, "disk"
	}
	return true, ""
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

	// TODO: remove idset
	cores := idset.From[uint16](c.ReservedCores)
	otherCores := idset.From[uint16](other.ReservedCores)
	if len(c.ReservedCores) > 0 && !cores.Superset(otherCores) {
		return false
	}

	return true
}

// TODO: Remove the indirection from this shallow struct.
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
	SharedPorts       AllocatedPorts // TODO?
}

func (c *ComparableNetworks) Add(delta *ComparableNetworks) {
	if delta == nil {
		return
	}
}

func (c *ComparableNetworks) Superset(other *ComparableNetworks) bool {
	// TODO unimplemented
	return false
}

type ComparableDevices struct {
	Devices []*AllocatedDeviceResource
}

func (c *ComparableDevices) Add(delta *ComparableDevices) {
	if delta == nil {
		return
	}
}

func (c *ComparableDevices) Superset(other *ComparableDevices) bool {
	// TODO unimplemented
	return false
}
