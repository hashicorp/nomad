// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

type QueueStatusRequest struct {
	Object string `json:"object,omitempty"`
	QueryOptions
}

type DynamicPriorityWorkload struct {
	JobID            string
	Tenant           string
	AdjustedPriority int
	BasePriority     int
	UsageAdjustment  int
	AgeAdjustment    int
	SizeAdjustment   int
}

type DynamicPriorityTenant struct {
	TenantID       string
	PercentageUsed int
	TenantUsage    map[string]float64
	TotalUsage     map[string]float64
}

type QueueStatusResponse struct {
	Type BatchQueueType

	// Results contains data about a specific queue
	// that is important to a consumer of this API.
	// The actual type is based on the "Type" parameter.
	Results any
	QueryMeta
}
