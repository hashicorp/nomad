// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
)

func TestNodePool_Copy(t *testing.T) {
	ci.Parallel(t)

	pool := &NodePool{
		Name:        "original",
		Description: "original node pool",
		Meta:        map[string]string{"original": "true"},
		SchedulerConfiguration: &NodePoolSchedulerConfiguration{
			SchedulerAlgorithm:            SchedulerAlgorithmSpread,
			MemoryOversubscriptionEnabled: pointer.Of(false),
		},
	}
	poolCopy := pool.Copy()
	poolCopy.Name = "copy"
	poolCopy.Description = "copy of original pool"
	poolCopy.Meta["original"] = "false"
	poolCopy.Meta["new_key"] = "true"
	poolCopy.SchedulerConfiguration.SchedulerAlgorithm = SchedulerAlgorithmBinpack
	poolCopy.SchedulerConfiguration.MemoryOversubscriptionEnabled = pointer.Of(true)

	must.NotEq(t, pool, poolCopy)
	must.NotEq(t, pool.Meta, poolCopy.Meta)
	must.NotEq(t, pool.SchedulerConfiguration, poolCopy.SchedulerConfiguration)
}

func TestNodePool_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name        string
		pool        *NodePool
		expectedErr string
	}{
		{
			name: "valid pool",
			pool: &NodePool{
				Name:        "valid",
				Description: "ok",
			},
		},
		{
			name: "invalid pool name character",
			pool: &NodePool{
				Name: "not-valid-😢",
			},
			expectedErr: "invalid name",
		},
		{
			name: "missing pool name",
			pool: &NodePool{
				Name: "",
			},
			expectedErr: "invalid name",
		},
		{
			name: "invalid pool description",
			pool: &NodePool{
				Name:        "valid",
				Description: strings.Repeat("a", 300),
			},
			expectedErr: "description longer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.pool.Validate()

			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)
			}
		})
	}
}

func TestNodePool_IsBuiltIn(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name    string
		pool    *NodePool
		builtIn bool
	}{
		{
			name: "all",
			pool: &NodePool{
				Name: NodePoolAll,
			},
			builtIn: true,
		},
		{
			name: "default",
			pool: &NodePool{
				Name: NodePoolDefault,
			},
			builtIn: true,
		},
		{
			name: "not built-in",
			pool: &NodePool{
				Name: "not-built-in",
			},
			builtIn: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pool.IsBuiltIn()
			must.Eq(t, tc.builtIn, got)
		})
	}
}

func TestNodePool_MemoryOversubscriptionEnabled(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		pool     *NodePool
		global   *SchedulerConfiguration
		expected bool
	}{
		{
			name: "global used if pool is nil",
			pool: nil,
			global: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: true,
			},
			expected: true,
		},
		{
			name: "global used if pool doesn't have scheduler config",
			pool: &NodePool{},
			global: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: true,
			},
			expected: true,
		},
		{
			name: "global used if pool doesn't specify memory oversub",
			pool: &NodePool{
				SchedulerConfiguration: &NodePoolSchedulerConfiguration{},
			},
			global: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: true,
			},
			expected: true,
		},
		{
			name: "pool overrides global if it defines memory oversub",
			pool: &NodePool{
				SchedulerConfiguration: &NodePoolSchedulerConfiguration{
					MemoryOversubscriptionEnabled: pointer.Of(false),
				},
			},
			global: &SchedulerConfiguration{
				MemoryOversubscriptionEnabled: true,
			},
			expected: false,
		},
		{
			name: "pool used if global is nil",
			pool: &NodePool{
				SchedulerConfiguration: &NodePoolSchedulerConfiguration{
					MemoryOversubscriptionEnabled: pointer.Of(true),
				},
			},
			global:   nil,
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pool.MemoryOversubscriptionEnabled(tc.global)
			must.Eq(t, got, tc.expected)
		})
	}
}
