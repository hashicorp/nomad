// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"errors"
	"fmt"
	"time"
)

type (
	BatchQueueTenant string
	BatchQueueType   string
)

const (
	BatchQueueTenantMetadata  BatchQueueTenant = "metadata"
	BatchQueueTenantNamespace BatchQueueTenant = "namespace"

	BatchQueueTypeDynamic     BatchQueueType = "dynamic_priority"
	BatchQueueTypeFifo        BatchQueueType = "fifo"
	BatchQueueTypePassthrough BatchQueueType = "passthrough"
)

type BatchQueue struct {
	client *Client
}

func (c *Client) BatchQueue() *BatchQueue {
	return &BatchQueue{client: c}
}

type DynamicPriorityWorkload struct {
	JobID               string
	Tenant              string
	Status              string
	Position            int
	AdjustedPriority    int
	BasePriority        int
	FairshareAdjustment int
	AgeAdjustment       int
	CpuAdjustment       int
	MemoryAdjustment    int
	CreatedAt           int64
}

type DynamicPriorityTenant struct {
	TenantID        string
	PercentageUsed  int
	TenantFairshare map[string]float64
	TotalFairshare  map[string]float64
}

type Workload struct {
	JobID     string
	Position  int
	Status    string
	CreatedAt int64
}

type BatchJobQueueJobsResponse struct {
	Type BatchQueueType
	// Workloads contains data about a specific queue
	// that is important to a consumer of this API.
	// The struct type is based on the "Type" parameter.
	Workloads any
}

type BatchJobQueueTenantsResponse struct {
	Type BatchQueueType
	// Tenants contains data about a specific queue
	// that is important to a consumer of this API.
	// The struct type is based on the "Type" parameter.
	Tenants any
}

// Jobs is used to query the current batch job queue.
func (q *BatchQueue) Jobs(queryOpts *QueryOptions) (*BatchJobQueueJobsResponse, *QueryMeta, error) {
	var resp BatchJobQueueJobsResponse
	endpoint := "/v1/queue/jobs"

	qm, err := q.client.query(endpoint, &resp, queryOpts)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

// Tenants is used to query the current batch job queue.
func (q *BatchQueue) Tenants(queryOpts *QueryOptions) (*BatchJobQueueTenantsResponse, *QueryMeta, error) {
	var resp BatchJobQueueTenantsResponse
	endpoint := "/v1/queue/tenants"

	qm, err := q.client.query(endpoint, &resp, queryOpts)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

// BatchQueueConfig configures a batch job queue for a node pool.
// If it is absent from NodePool config, the pool's batch job queue is disabled.
// One and only one queue config must be set.
type BatchQueueConfig struct {
	DynamicPriority *DynamicQueueConfig `hcl:"dynamic_priority,block"`
	Fifo            *FifoQueueConfig    `hcl:"fifo,block"`
}

// TenantFairshareConfig configures how tenants are identified and weighted.
type TenantFairshareConfig struct {
	// TenantType determines how jobs are categorized into tenants,
	// may be either "namespace" or "metadata". If "metadata" is used,
	// MetadataKey must be specified.
	TenantType BatchQueueTenant `hcl:"tenant_type"`

	// MetadataKey specifies the key used in job meta{} block.
	// Each unique value is treated as a separate tenant.
	// Only valid with TenantType = "metadata"
	MetadataKey string `hcl:"metadata_key,optional"`

	// CpuWeight determines how much a tenant's cpu usage affects
	// the priority of all of its queued jobs.
	CpuWeight int `hcl:"cpu_weight"`

	// MemoryWeight determines how much a tenant's memory usage affects
	// the priority of all of its queued jobs.
	MemoryWeight int `hcl:"memory_weight"`

	// ExcludeAllocStatuses defines the alloc statuses to exclude from
	// tenant resource usage calculations.
	ExcludeAllocStatuses []string `hcl:"exclude_alloc_statuses,optional"`
}

// AgeConfig configures how job age affects scheduling priority.
type AgeConfig struct {
	// Weight determines how much the job's age affects its priority.
	Weight int `hcl:"age_weight,optional"`
	// MaxAge is the top end of the age calculation for a job, past which the age weight is capped.
	MaxAge time.Duration `hcl:"max_age,optional"`
}

// JobSizeConfig configures how job resource size affects scheduling priority.
type JobSizeConfig struct {
	// CpuWeight determines how much a job's requested cpu affects its priority.
	CpuWeight int `hcl:"cpu_weight,optional"`
	// MaxCpu is the top end of the cpu value for a job, past which the cpu weight is capped.
	MaxCpu int `hcl:"max_cpu,optional"`

	// MemoryWeight determines how much a job's requested mem affects its priority.
	MemoryWeight int `hcl:"memory_weight,optional"`
	// MaxMemory is the top end of the memory value for a job, past which the memory weight is capped.
	MaxMemory int `hcl:"max_memory,optional"`
}

// DynamicQueueConfig configures a dynamic priority queue for a node pool.
type DynamicQueueConfig struct {
	// CalcInterval is how often the queue will recalculate priorities.
	CalcInterval time.Duration `hcl:"calc_interval,optional"`

	// TODO: validate and set sensible defaults.
	TenantFairshare TenantFairshareConfig `hcl:"tenant_fairshare,block"`
	Age             AgeConfig             `hcl:"age,block"`
	JobSize         JobSizeConfig         `hcl:"job_size,block"`
}

// FifoQueueConfig enables a FIFO queue for a node pool.
type FifoQueueConfig struct{}
