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
	JobID            string
	Tenant           string
	Position         int
	AdjustedPriority int
	BasePriority     int
	UsageAdjustment  int
	AgeAdjustment    int
	SizeAdjustment   int
	CreatedAt        int64
}

type DynamicPriorityTenant struct {
	TenantID       string
	PercentageUsed int
	TenantUsage    map[string]float64
	TotalUsage     map[string]float64
}

type Workload struct {
	JobID     string
	Position  int
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

// Validate provides some minimal client-side validation of the queue config.
func (b *BatchQueueConfig) Validate() error {
	if (b.DynamicPriority == nil && b.Fifo == nil) ||
		(b.DynamicPriority != nil && b.Fifo != nil) {
		return fmt.Errorf("must specify a single queue config; dynamic_priority or fifo")
	}

	if b.DynamicPriority != nil {
		if err := b.DynamicPriority.Validate(); err != nil {
			return fmt.Errorf("dynamic_priority config is invalid: %w", err)
		}
	}

	return nil
}

// DynamicQueueConfig configures a dynamic priority queue for a node pool.
type DynamicQueueConfig struct {
	// TenantType determines how jobs are categorized into tenants,
	// may be either "namespace" or "metadata". If "metadata" is used,
	// MetadataKey must be specified.
	TenantType BatchQueueTenant `hcl:"tenant_type"`

	// MetadataKey specifies the key used in job meta{} block.
	// Each unique value is treated as a separate tenant.
	// Only valid with TenantType = "metadata"
	MetadataKey string `hcl:"metadata_key"`

	// CalcInterval is how often the queue will recalculate priorities.
	CalcInterval time.Duration `hcl:"calc_interval"`

	// AgeWeight determines how much the job's age affects its priority.
	AgeWeight int `hcl:"age_weight"`
	// MaxAge is the top end of the age calculation for a job,
	// past which the age weight is capped.
	MaxAge time.Duration `hcl:"max_age"`

	// UsageWeight determines ...
	UsageWeight int `hcl:"usage_weight"`
	// HalfLife determines the rate at which we decay the impact of a job's
	// resource usage over time.
	HalfLife time.Duration `hcl:"half_life"`

	// SizeWeight ... TODO: mike made this obsolete
	SizeWeight int `hcl:"size_weight"`
	// MaxSize ... ditto ^
	MaxSize int `hcl:"max_size"`
}

func (qc *DynamicQueueConfig) Validate() error {
	switch qc.TenantType {
	case BatchQueueTenantNamespace:
	case BatchQueueTenantMetadata:
		if qc.MetadataKey == "" {
			return errors.New("metadata key must be specified if using metadata tenency")
		}
	case "":
		return errors.New("tenant type must be specified if using dynamic priority queue")
	default:
		return fmt.Errorf("unsupported tenant type: %q", qc.TenantType)
	}

	if qc.CalcInterval <= 0 {
		return errors.New("calc_interval must be greater than zero")
	}
	if qc.HalfLife <= 0 {
		return errors.New("half_life must be greater than zero")
	}

	return nil
}

// FifoQueueConfig enables a FIFO queue for a node pool.
type FifoQueueConfig struct{}
