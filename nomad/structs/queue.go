// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

type QueueWorkload interface {
	GetID() string
	GetCreateIndex() uint64
	GetNamespace() string
	// Stub() (QueueWorkload, error)
}

type DynamicPriorityWorkload struct {
	JobID            string
	Tenant           string
	Namespace        string
	Position         int
	AdjustedPriority int
	BasePriority     int
	UsageAdjustment  int
	AgeAdjustment    int
	SizeAdjustment   int
	CreatedAt        int64
}

func (w *DynamicPriorityWorkload) GetID() string {
	return w.JobID
}

func (w *DynamicPriorityWorkload) GetCreateIndex() uint64 {
	return uint64(w.CreatedAt)
}

func (w *DynamicPriorityWorkload) GetNamespace() string {
	return w.Namespace
}

func (w *DynamicPriorityWorkload) Stub() (QueueWorkload, error) {
	return w, nil
}

type DynamicPriorityTenant struct {
	TenantID       string
	PercentageUsed int
	TenantUsage    map[string]float64
	TotalUsage     map[string]float64
}

type QueueTenantsRequest struct {
	QueryOptions
}

type QueueTenantsResponse struct {
	Type BatchQueueType

	// Tenants contains data about a specific queue
	// that is important to a consumer of this API.
	// The actual type is based on the "Type" parameter.
	Tenants any
	QueryMeta
}

type QueueJobsRequest struct {
	QueryOptions
}

type QueueJobsResponse struct {
	Type BatchQueueType

	// Workloads contains data about a specific queue
	// that is important to a consumer of this API.
	// The actual type is based on the "Type" parameter.
	Workloads any
	QueryMeta
}
