// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskenv

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func Test_InterpolateNetworks(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		inputTaskEnv           *TaskEnv
		inputNetworks          structs.Networks
		expectedOutputNetworks structs.Networks
		name                   string
	}{
		{
			inputTaskEnv: testEnv,
			inputNetworks: structs.Networks{
				{Hostname: "my-little-pony"},
			},
			expectedOutputNetworks: structs.Networks{
				{Hostname: "my-little-pony"},
			},
			name: "non-interpolated hostname",
		},
		{
			inputTaskEnv: testEnv,
			inputNetworks: structs.Networks{
				{Hostname: "${foo}-cache-${baz}"},
			},
			expectedOutputNetworks: structs.Networks{
				{Hostname: "bar-cache-blah"},
			},
			name: "interpolated hostname",
		},
		{
			inputTaskEnv: testEnv,
			inputNetworks: structs.Networks{
				{
					DNS: &structs.DNSConfig{
						Servers:  []string{"127.0.0.1"},
						Options:  []string{"some-opt"},
						Searches: []string{"example.com"},
					},
				},
			},
			expectedOutputNetworks: structs.Networks{
				{
					DNS: &structs.DNSConfig{
						Servers:  []string{"127.0.0.1"},
						Options:  []string{"some-opt"},
						Searches: []string{"example.com"},
					},
				},
			},
			name: "non-interpolated dns servers",
		},
		{
			inputTaskEnv: testEnv,
			inputNetworks: structs.Networks{
				{
					DNS: &structs.DNSConfig{
						Servers:  []string{"${foo}"},
						Options:  []string{"${foo}-opt"},
						Searches: []string{"${foo}.example.com"},
					},
				},
			},
			expectedOutputNetworks: structs.Networks{
				{
					DNS: &structs.DNSConfig{
						Servers:  []string{"bar"},
						Options:  []string{"bar-opt"},
						Searches: []string{"bar.example.com"},
					},
				},
			},
			name: "interpolated dns servers",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := InterpolateNetworks(tc.inputTaskEnv, tc.inputNetworks)
			assert.Equal(t, tc.expectedOutputNetworks, actualOutput, tc.name)
		})
	}
}
