// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package serviceregistration

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func Test_MakeAllocServiceID(t *testing.T) {
	testCases := []struct {
		inputAllocID   string
		inputTaskName  string
		inputService   *structs.Service
		expectedOutput string
		name           string
	}{
		{
			inputAllocID:  "7ac7c672-1824-6f06-644c-4c249e1578b9",
			inputTaskName: "cache",
			inputService: &structs.Service{
				Name:      "redis",
				PortLabel: "db",
			},
			expectedOutput: "_nomad-task-7ac7c672-1824-6f06-644c-4c249e1578b9-cache-redis-db",
			name:           "generic 1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := MakeAllocServiceID(tc.inputAllocID, tc.inputTaskName, tc.inputService)
			require.Equal(t, tc.expectedOutput, actualOutput)
		})
	}
}
