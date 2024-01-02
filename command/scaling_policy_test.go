// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/assert"
)

func Test_formatScalingPolicyTarget(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		inputMap       map[string]string
		expectedOutput string
		name           string
	}{
		{
			inputMap: map[string]string{
				"Namespace": "default",
				"Job":       "example",
				"Group":     "cache",
			},
			expectedOutput: "Namespace:default,Job:example,Group:cache",
			name:           "generic horizontal scaling policy target",
		},
		{
			inputMap: map[string]string{
				"Namespace": "default",
				"Job":       "example",
				"Group":     "cache",
				"Unknown":   "alien",
			},
			expectedOutput: "Namespace:default,Job:example,Group:cache,Unknown:alien",
			name:           "extra key in input mapping",
		},
		{
			inputMap: map[string]string{
				"Namespace": "default",
			},
			expectedOutput: "Namespace:default",
			name:           "single entry in map",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := formatScalingPolicyTarget(tc.inputMap)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}
