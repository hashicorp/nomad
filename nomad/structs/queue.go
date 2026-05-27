// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

type QueueStatusRequest struct {
	QueryOptions
}

type DynamicPriorityWorkload struct {
	JobID            string
	Tenant           string
	AdjustedPriority int
	BasePriority     int
	UsageAjustment   int
	AgeAdjustment    int
	SizeAdjustment   int
}

type QueueStatusResponse struct {
	Type BatchQueueType

	// Workloads are the actual queue workloads
	// where their actual type is based on the
	// "Type" parameter above.
	Workloads any
	QueryMeta
}
