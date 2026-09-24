// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestJobs_BatchQueue_Jobs(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	queue := c.BatchQueue()

	resp, _, err := queue.Jobs(nil)
	must.NoError(t, err)
	must.Eq(t, "passthrough", resp.Type)
}

func TestJobs_BatchQueue_Tenants(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	queue := c.BatchQueue()

	resp, _, err := queue.Tenants(nil)
	must.NoError(t, err)
	must.Eq(t, "passthrough", resp.Type)
}

func TestBatchQueueConfig_Validate(t *testing.T) {
	testutil.Parallel(t)

	testCases := []struct {
		name   string
		config *BatchQueueConfig
		err    string
	}{
		{
			name:   "missing queue config",
			config: &BatchQueueConfig{},
			err:    "must specify a single queue config",
		},
		{
			name: "multiple queue configs",
			config: &BatchQueueConfig{
				DynamicPriority: &DynamicQueueConfig{},
				Fifo:            &FifoQueueConfig{},
			},
			err: "must specify a single queue config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
			} else {
				must.NoError(t, err)
			}
		})
	}
}

func TestBatchQueueConfig_DynamicQueueConfig_Validate(t *testing.T) {
	testutil.Parallel(t)

	testCases := []struct {
		name   string
		config *DynamicQueueConfig
		err    string
	}{
		{
			name:   "missing tenant type",
			config: &DynamicQueueConfig{},
			err:    "tenant type must be specified",
		},
		{
			name: "bad tenant type",
			config: &DynamicQueueConfig{
				TenantFairshare: struct {
					TenantType           BatchQueueTenant `hcl:"tenant_type"`
					MetadataKey          string           `hcl:"metadata_key,optional"`
					CpuWeight            int              `hcl:"cpu_weight"`
					MemoryWeight         int              `hcl:"memory_weight"`
					ExcludeAllocStatuses []string         `hcl:"exclude_alloc_statuses,optional"`
				}{TenantType: "foo"},
			},
			err: "unsupported tenant type",
		},
		{
			name: "bad missing metadata key",
			config: &DynamicQueueConfig{
				TenantFairshare: struct {
					TenantType           BatchQueueTenant `hcl:"tenant_type"`
					MetadataKey          string           `hcl:"metadata_key,optional"`
					CpuWeight            int              `hcl:"cpu_weight"`
					MemoryWeight         int              `hcl:"memory_weight"`
					ExcludeAllocStatuses []string         `hcl:"exclude_alloc_statuses,optional"`
				}{TenantType: BatchQueueTenantMetadata},
			},
			err: "metadata key must be specified",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
			} else {
				must.NoError(t, err)
			}
		})
	}
}
