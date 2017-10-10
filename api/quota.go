package api

// QuotaSpec specifies the allowed resource usage across regions.
type QuotaSpec struct {
	// Name is the name for the quota object
	Name string

	// Description is an optional description for the quota object
	Description string

	// Limits is the set of quota limits encapsulated by this quota object. Each
	// limit applies quota in a particular region and in the future over a
	// particular priority range and datacenter set.
	Limits []*QuotaLimit

	// Raft indexes to track creation and modification
	CreateIndex uint64
	ModifyIndex uint64
}

// QuotaLimit describes the resource limit in a particular region.
type QuotaLimit struct {
	// Region is the region in which this limit has affect
	Region string

	// RegionLimit is the quota limit that applies to any allocation within a
	// referencing namespace in the region. A value of zero is treated as
	// unlimited and a negative value is treated as fully disallowed. This is
	// useful for once we support GPUs
	RegionLimit *Resources

	// Hash is the hash of the object and is used to make replication efficient.
	Hash []byte
}

// QuotaUsage is the resource usage of a Quota
type QuotaUsage struct {
	Name        string
	Used        map[string]*QuotaLimit
	CreateIndex uint64
	ModifyIndex uint64
}
