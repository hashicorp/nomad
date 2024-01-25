package structs

type PortlandRequest struct {
	WriteRequest

	UpsertAllocs []*Allocation
	UpsertJobs   []*Job

	DeleteAllocs     []string
	DeleteJobs       []NamespacedID
	DeleteNodes      []string
	DeleteNamespaces []string
}
