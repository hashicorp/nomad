// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1
package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestValidateConstraintTarget(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name             string
		inputTarget      string
		expectedErrorMsg string
	}{
		{
			name:             "literal value",
			inputTarget:      "literal-value",
			expectedErrorMsg: "",
		},
		{
			name:             "literal empty",
			inputTarget:      "",
			expectedErrorMsg: "",
		},
		{
			name:             "valid node.unique.id",
			inputTarget:      "${node.unique.id}",
			expectedErrorMsg: "",
		},
		{
			name:             "valid node.datacenter",
			inputTarget:      "${node.datacenter}",
			expectedErrorMsg: "",
		},
		{
			name:             "valid node.unique.name",
			inputTarget:      "${node.unique.name}",
			expectedErrorMsg: "",
		},
		{
			name:             "valid node.class",
			inputTarget:      "${node.class}",
			expectedErrorMsg: "",
		},
		{
			name:             "valid node.pool",
			inputTarget:      "${node.pool}",
			expectedErrorMsg: "",
		},
		{
			name:             "valid attr.kernel.name",
			inputTarget:      "${attr.kernel.name}",
			expectedErrorMsg: "",
		},
		{
			name:             "valid attr.cpu.arch",
			inputTarget:      "${attr.cpu.arch}",
			expectedErrorMsg: "",
		},
		{
			name:             "invalid exact with suffix",
			inputTarget:      "${node.pool.}",
			expectedErrorMsg: `unsupported attribute "${node.pool.}"`,
		},
		{
			name:             "valid meta.team",
			inputTarget:      "${meta.team}",
			expectedErrorMsg: "",
		},
		{
			name:             "valid meta.environment",
			inputTarget:      "${meta.environment}",
			expectedErrorMsg: "",
		},
		{
			name:             "valid device.vendor",
			inputTarget:      "${device.vendor}",
			expectedErrorMsg: "",
		},
		{
			name:             "valid device.type",
			inputTarget:      "${device.type}",
			expectedErrorMsg: "",
		},
		{
			name:             "missing closing brace",
			inputTarget:      "${node.datacenter",
			expectedErrorMsg: `attribute "${node.datacenter" is missing a closing brace`,
		},
		{
			name:             "unsupported interpolated value",
			inputTarget:      "${env.PATH}",
			expectedErrorMsg: `unsupported attribute "${env.PATH}"`,
		},
		{
			name:             "invalid node attribute",
			inputTarget:      "${node.invalid}",
			expectedErrorMsg: `unsupported attribute "${node.invalid}"`,
		},
		{
			name:             "empty interpolation",
			inputTarget:      "${}",
			expectedErrorMsg: `unsupported attribute "${}"`,
		},
		{
			name:             "missing starting brace",
			inputTarget:      "$node.datacenter}",
			expectedErrorMsg: `attribute "$node.datacenter}" is missing an opening brace`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actualErr := validateConstraintAttribute(tc.inputTarget)
			if tc.expectedErrorMsg != "" {
				must.ErrorContains(t, actualErr, tc.expectedErrorMsg)
			} else {
				must.NoError(t, actualErr)
			}
		})
	}
}
