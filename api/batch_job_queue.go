// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package api

type BatchJobQueue struct {
	client *Client
}

func (c *Client) BatchJobQueue() *BatchJobQueue {
	return &BatchJobQueue{client: c}
}

// BatchJobQueue is the configuration for a batch job queue used to control scheduling
// of batch jobs.
type (
	BatchJobQueueTenant string
	BatchJobQueueType   string
)

const (
	BatchJobQueueTypeDynamic BatchJobQueueType = "dynamic_priority"

	BatchJobQueueTenantMetadata  BatchJobQueueTenant = "metadata"
	BatchJobQueueTenantNamespace BatchJobQueueTenant = "namespace"
)

type BatchJobQueueConfig struct {
	Type        BatchJobQueueType
	TenantType  BatchJobQueueTenant
	MetadataKey string
	Config      map[string]any
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

type BatchJobQueueStatusResponse struct {
	Type BatchJobQueueType
	// Workloads contains data about a specific queue
	// that is important to a consumer of this API.
	// The struct type is based on the "Type" parameter.
	Workloads any
}

type BatchJobQueueStatusOptions struct{}

// Status is used to query the current batch job queue.
func (q *BatchJobQueue) Status(opts *BatchJobQueueStatusOptions, queryOpts *QueryOptions) (*BatchJobQueueStatusResponse, *QueryMeta, error) {
	var resp BatchJobQueueStatusResponse
	qm, err := q.client.query("/v1/queue/status", &resp, queryOpts)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}
