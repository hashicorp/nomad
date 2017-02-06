package api

import "github.com/hashicorp/nomad/helper"

// Resources encapsulates the required resources of
// a given task or task group.
type Resources struct {
	CPU      *int
	MemoryMB *int
	DiskMB   *int
	IOPS     *int
	Networks []*NetworkResource
}

func MinResources() *Resources {
	return &Resources{
		CPU:      helper.IntToPtr(100),
		MemoryMB: helper.IntToPtr(10),
		IOPS:     helper.IntToPtr(0),
	}

}

// Merge merges this resource with another resource.
func (r *Resources) Merge(other *Resources) {
	if other == nil {
		return
	}
	if other.CPU != nil {
		r.CPU = other.CPU
	}
	if other.MemoryMB != nil {
		r.MemoryMB = other.MemoryMB
	}
	if other.DiskMB != nil {
		r.DiskMB = other.DiskMB
	}
	if other.IOPS != nil {
		r.IOPS = other.IOPS
	}
	if len(other.Networks) != 0 {
		r.Networks = other.Networks
	}
}

type Port struct {
	Label string
	Value int
}

// NetworkResource is used to describe required network
// resources of a given task.
type NetworkResource struct {
	Public        bool
	CIDR          string
	ReservedPorts []Port
	DynamicPorts  []Port
	IP            string
	MBits         int
}
