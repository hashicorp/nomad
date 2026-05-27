// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package api

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
}

type BatchQueueStatusOptions struct{}

// BatchQueueStatus is used to query the current batch job queue.
func (j *Jobs) BatchQueueStatus(opts *BatchQueueStatusOptions, q *QueryOptions) (*QueueStatusResponse, *QueryMeta, error) {
	var resp QueueStatusResponse
	qm, err := j.client.query("/v1/jobs/queue/status", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}
