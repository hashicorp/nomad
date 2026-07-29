package structs

type ComparableCpuMemDisk struct {
	*ComparableCPU
	*ComparableMem
	*ComparableDisk
}

func (c *ComparableCpuMemDisk) Superset(other *ComparableCpuMemDisk) bool {
	return c.ComparableCPU.Superset(other.ComparableCPU) &&
		c.ComparableMem.Superset(other.ComparableMem) &&
		c.ComparableDisk.Superset(other.ComparableDisk)
}

// Comparable's are resources allocated to a task group and not keyed
// by Task, making them easier to compare.

// TODO: we can remove the indirection from these shallow structs
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

func (c *ComparableCPU) Superset(other *ComparableCPU) bool {
	// TODO unimplemented
	return false
}

// TODO: we can remove the indirection from these shallow structs
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

func (c *ComparableMem) Superset(other *ComparableMem) bool {
	// TODO Unimplemented
	return false
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

func (c *ComparableDisk) Superset(other *ComparableDisk) bool {
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
