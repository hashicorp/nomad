// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
)

func TestSchedulerConfiguration_GetNodeLimitForFeasibilityChecks(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		config   *SchedulerConfiguration
		expected uint
	}{
		{
			name:     "nil config returns default",
			config:   nil,
			expected: DefaultNodeLimitForFeasibilityChecks,
		},
		{
			name:     "zero value returns default",
			config:   &SchedulerConfiguration{},
			expected: DefaultNodeLimitForFeasibilityChecks,
		},
		{
			name: "positive value is returned",
			config: &SchedulerConfiguration{
				NodeLimitForFeasibilityChecks: 42,
			},
			expected: 42,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expected, tc.config.GetNodeLimitForFeasibilityChecks())
		})
	}
}

func TestSchedulerConfiguration_WithNodePool(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name        string
		schedConfig *SchedulerConfiguration
		pool        *NodePool
		expected    *SchedulerConfiguration
	}{
		{
			name: "nil pool returns same config",
			schedConfig: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: false,
				SchedulerAlgorithm:            SchedulerAlgorithmSpread,
			},
			pool: nil,
			expected: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: false,
				SchedulerAlgorithm:            SchedulerAlgorithmSpread,
			},
		},
		{
			name: "nil pool scheduler config returns same config",
			schedConfig: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: false,
				SchedulerAlgorithm:            SchedulerAlgorithmSpread,
			},
			pool: &NodePool{},
			expected: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: false,
				SchedulerAlgorithm:            SchedulerAlgorithmSpread,
			},
		},
		{
			name: "pool with memory oversubscription overwrites config",
			schedConfig: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: false,
			},
			pool: &NodePool{
				SchedulerConfiguration: &NodePoolSchedulerConfiguration{
					MemoryOversubscriptionEnabled: pointer.Of(true),
					BatchQueue: BatchQueue{
						Type: "test",
					},
				},
			},
			expected: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: true,
				BatchQueue: BatchQueue{
					Type: "test",
				},
			},
		},
		{
			name: "pool with scheduler algorithm overwrites config",
			schedConfig: &SchedulerConfiguration{
				SchedulerAlgorithm: SchedulerAlgorithmBinpack,
			},
			pool: &NodePool{
				SchedulerConfiguration: &NodePoolSchedulerConfiguration{
					SchedulerAlgorithm: SchedulerAlgorithmSpread,
				},
			},
			expected: &SchedulerConfiguration{
				SchedulerAlgorithm: SchedulerAlgorithmSpread,
			},
		},
		{
			name: "pool without memory oversubscription does not modify config",
			schedConfig: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: false,
			},
			pool: &NodePool{
				SchedulerConfiguration: &NodePoolSchedulerConfiguration{},
			},
			expected: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: false,
			},
		},
		{
			name: "pool without scheduler algorithm does not modify config",
			schedConfig: &SchedulerConfiguration{
				SchedulerAlgorithm: SchedulerAlgorithmSpread,
			},
			pool: &NodePool{
				SchedulerConfiguration: &NodePoolSchedulerConfiguration{},
			},
			expected: &SchedulerConfiguration{
				SchedulerAlgorithm: SchedulerAlgorithmSpread,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.schedConfig.WithNodePool(tc.pool)
			must.Eq(t, tc.expected, got)
			must.NotEqOp(t, tc.schedConfig, got)
		})
	}
}

func TestSchedulerConfiguration_Validate(t *testing.T) {

	testCases := []struct {
		name        string
		schedConfig *SchedulerConfiguration
		err         string
	}{
		{
			name: "invalid scheduler algorithm",
			schedConfig: &SchedulerConfiguration{
				SchedulerAlgorithm: "not-good",
			},
			err: "invalid scheduler algorithm: not-good",
		},
		{
			name: "valid scheduler algorithm",
			schedConfig: &SchedulerConfiguration{
				SchedulerAlgorithm: SchedulerAlgorithmBinpack,
			},
			err: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.schedConfig.Validate()
			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
			} else {
				must.NoError(t, err)
			}
		})
	}
}

func TestBatchQueue_Validate(t *testing.T) {

	testCases := []struct {
		name        string
		batchConfig BatchQueue
		err         string
	}{
		{
			name: "invalid queue type",
			batchConfig: BatchQueue{
				Type: "foo",
			},
			err: "unsupported batch queue type",
		},
		{
			name: "invalid metadata type",
			batchConfig: BatchQueue{
				Type:       BatchQueueTypeDynamic,
				TenantType: "foo",
			},
			err: "unsupported tenant type",
		},
		{
			name: "batch config with no type",
			batchConfig: BatchQueue{
				Type:       "",
				TenantType: TenantTypeNamespace,
			},
			err: "batch queue configuration found but no type specified",
		},
		{
			name: "empty metadata key errors",
			batchConfig: BatchQueue{
				Type:       BatchQueueTypeDynamic,
				TenantType: TenantTypeMetadata,
				Config: map[string]any{
					"calc_interval": "1s",
					"half_life":     "1s",
				},
			},
			err: "metadata key must be specified",
		},
		{
			name: "dynamic_priority - invalid interval",
			batchConfig: BatchQueue{
				Type:       BatchQueueTypeDynamic,
				TenantType: TenantTypeNamespace,
				Config: map[string]any{
					"calc_interval": "hello",
				},
			},
			err: "unable to decode conf",
		},
		{
			name: "dynamic_priority - valid string interval",
			batchConfig: BatchQueue{
				Type:       BatchQueueTypeDynamic,
				TenantType: TenantTypeNamespace,
				Config: map[string]any{
					"calc_interval": "1h",
					"half_life":     "1h",
				},
			},
			err: "",
		},
		{
			name: "dynamic_priority - valid int interval",
			batchConfig: BatchQueue{
				Type:       BatchQueueTypeDynamic,
				TenantType: TenantTypeNamespace,
				Config: map[string]any{
					"calc_interval": 1000,
					"half_life":     "1h",
				},
			},
			err: "",
		},
		{
			name: "dynamicPriority - zero calc interval",
			batchConfig: BatchQueue{
				Type:       BatchQueueTypeDynamic,
				TenantType: TenantTypeNamespace,
				Config: map[string]any{
					"calc_interval": 0,
					"half_life":     "1s",
				},
			},
			err: "calc_interval must be greater than zero",
		},
		{
			name: "dynamicPriority - zero half life",
			batchConfig: BatchQueue{
				Type:       BatchQueueTypeDynamic,
				TenantType: TenantTypeNamespace,
				Config: map[string]any{
					"calc_interval": "1s",
					"half_life":     0,
				},
			},
			err: "half_life must be greater than zero",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.batchConfig.Validate()
			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
			} else {
				must.NoError(t, err)
			}
		})
	}
}
