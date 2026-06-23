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

	BatchQueueObjectTenants = "tenants"
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
	Position         int
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

type BatchJobQueueJobsResponse struct {
	Type BatchJobQueueType
	// Workloads contains data about a specific queue
	// that is important to a consumer of this API.
	// The struct type is based on the "Type" parameter.
	Workloads any
}

type BatchJobQueueTenantsResponse struct {
	Type BatchJobQueueType
	// Tenants contains data about a specific queue
	// that is important to a consumer of this API.
	// The struct type is based on the "Type" parameter.
	Tenants any
}

// Jobs is used to query the current batch job queue.
func (q *BatchJobQueue) Jobs(queryOpts *QueryOptions) (*BatchJobQueueJobsResponse, *QueryMeta, error) {
	var resp BatchJobQueueJobsResponse
	endpoint := "/v1/queue/jobs"

	qm, err := q.client.query(endpoint, &resp, queryOpts)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

// Tenants is used to query the current batch job queue.
func (q *BatchJobQueue) Tenants(queryOpts *QueryOptions) (*BatchJobQueueTenantsResponse, *QueryMeta, error) {
	var resp BatchJobQueueTenantsResponse
	endpoint := "/v1/queue/tenants"

	qm, err := q.client.query(endpoint, &resp, queryOpts)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}
