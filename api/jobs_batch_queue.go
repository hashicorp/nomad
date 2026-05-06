// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package api

type Workload struct {
	JobID    string
	Tenant   string
	Priority int
}

type BatchQueueStatusResponse struct {
	Workloads []Workload
}

// BatchQueueStatus is used to query the current batch job queue.
func (j *Jobs) BatchQueueStatus(q *QueryOptions) (*BatchQueueStatusResponse, *QueryMeta, error) {
	var resp BatchQueueStatusResponse
	qm, err := j.client.query("/v1/jobs/queue/status", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}
