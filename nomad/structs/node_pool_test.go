// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestNodePool_Copy(t *testing.T) {
	ci.Parallel(t)

	pool := &NodePool{
		Name:        "original",
		Description: "original node pool",
		Meta:        map[string]string{"original": "true"},
		SchedulerConfiguration: &NodePoolSchedulerConfiguration{
			SchedulerAlgorithm: SchedulerAlgorithmSpread,
		},
	}
	poolCopy := pool.Copy()
	poolCopy.Name = "copy"
	poolCopy.Description = "copy of original pool"
	poolCopy.Meta["original"] = "false"
	poolCopy.Meta["new_key"] = "true"
	poolCopy.SchedulerConfiguration.SchedulerAlgorithm = SchedulerAlgorithmBinpack

	must.NotEq(t, pool, poolCopy)
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
		{
			name: "invalid scheduling algorithm",
			pool: &NodePool{
				Name: "valid",
				SchedulerConfiguration: &NodePoolSchedulerConfiguration{
					SchedulerAlgorithm: "invalid",
				},
			},
			expectedErr: "invalid scheduler algorithm",
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

func TestNodePool_IsReserved(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		pool     *NodePool
		reserved bool
	}{
		{
			name: "all",
			pool: &NodePool{
				Name: NodePoolAll,
			},
			reserved: true,
		},
		{
			name: "default",
			pool: &NodePool{
				Name: NodePoolDefault,
			},
			reserved: true,
		},
		{
			name: "not reserved",
			pool: &NodePool{
				Name: "not-reserved",
			},
			reserved: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pool.IsReserved()
			must.Eq(t, tc.reserved, got)
		})
	}

}
