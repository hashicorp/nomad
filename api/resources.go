package api

// Resources encapsulates the required resources of
// a given task or task group.
type Resources struct {
	CPU      int
	MemoryMB int
	DiskMB   int
	IOPS     int
	Networks []*NetworkResource
}

// NetworkResource is used to describe required network
// resources of a given task.
type NetworkResource struct {
	Public        bool
	CIDR          string
	ReservedPorts []int
	DynamicPorts  []string
	MBits         int
}
