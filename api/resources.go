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

type Port struct {
	Label string
	Value int
}

type PortRange struct {
	Label string
	Base  int
	Span  int
}

// NetworkResource is used to describe required network
// resources of a given task.
type NetworkResource struct {
	Public            bool
	CIDR              string
	ReservedPorts     []Port
	DynamicPorts      []Port
	DynamicPortRanges []PortRange
	IP                string
	MBits             int
}
