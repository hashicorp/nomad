// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestNodePool_Validate_OSS(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name        string
		pool        *NodePool
		expectedErr string
	}{
		{
			name: "invalid scheduling algorithm",
			pool: &NodePool{
				Name: "valid",
				SchedulerConfiguration: &NodePoolSchedulerConfiguration{
					SchedulerAlgorithm: SchedulerAlgorithmBinpack,
				},
			},
			expectedErr: "unlicensed",
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
