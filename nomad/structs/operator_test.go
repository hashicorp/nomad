// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
)

func TestSchedulerConfiguration_GetNodeLimitForSpreadAndAffinity(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		config   *SchedulerConfiguration
		expected uint
	}{
		{
			name:     "nil config returns default",
			config:   nil,
			expected: DefaultNodeLimitForSpreadAndAffinity,
		},
		{
			name:     "zero value returns default",
			config:   &SchedulerConfiguration{},
			expected: DefaultNodeLimitForSpreadAndAffinity,
		},
		{
			name: "positive value is returned",
			config: &SchedulerConfiguration{
				NodeLimitForSpreadAndAffinity: 42,
			},
			expected: 42,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expected, tc.config.GetNodeLimitForSpreadAndAffinity())
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
				},
			},
			expected: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: true,
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
