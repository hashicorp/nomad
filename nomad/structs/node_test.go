// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestDriverInfoEquals(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	var driverInfoTest = []struct {
		input    []*DriverInfo
		expected bool
		errorMsg string
	}{
		{
			[]*DriverInfo{
				{
					Healthy: true,
				},
				{
					Healthy: false,
				},
			},
			false,
			"Different healthy values should not be equal.",
		},
		{
			[]*DriverInfo{
				{
					HealthDescription: "not running",
				},
				{
					HealthDescription: "running",
				},
			},
			false,
			"Different health description values should not be equal.",
		},
		{
			[]*DriverInfo{
				{
					Detected:          false,
					Healthy:           true,
					HealthDescription: "This driver is ok",
				},
				{
					Detected:          true,
					Healthy:           true,
					HealthDescription: "This driver is ok",
				},
			},
			true,
			"Same health check should be equal",
		},
	}
	for _, testCase := range driverInfoTest {
		first := testCase.input[0]
		second := testCase.input[1]
		require.Equal(testCase.expected, first.HealthCheckEquals(second), testCase.errorMsg)
	}
}

func TestNodeMeta_Validate(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		input    map[string]*string // only specify Meta field
		contains string
	}{
		{
			name: "Ok",
			input: map[string]*string{
				"foo":             nil,
				"bar":             nil,
				"eggs":            nil,
				"dots.are_ok-too": nil,
			},
		},
		{
			name:     "Nil",
			input:    nil,
			contains: "missing required",
		},
		{
			name:     "Empty",
			input:    map[string]*string{},
			contains: "missing required",
		},
		{
			name:     "EmptyKey",
			input:    map[string]*string{"": nil},
			contains: "not be empty",
		},
		{
			name: "Whitespace",
			input: map[string]*string{
				"ok":   nil,
				" bad": nil,
			},
			contains: `" bad" is invalid`,
		},
		{
			name: "BadChars",
			input: map[string]*string{
				"ok":    nil,
				"*bad%": nil,
			},
			contains: `"*bad%" is invalid`,
		},
		{
			name: "StartingDot",
			input: map[string]*string{
				"ok":   nil,
				".bad": nil,
			},
			contains: `".bad" is invalid`,
		},
		{
			name: "EndingDot",
			input: map[string]*string{
				"ok":   nil,
				"bad.": nil,
			},
			contains: `"bad." is invalid`,
		},
		{
			name: "DottedPartsMustBeValid",
			input: map[string]*string{
				"ok":        nil,
				"bad.-part": nil,
			},
			contains: `"bad.-part" is invalid`,
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			in := &NodeMetaApplyRequest{
				Meta: tc.input,
			}

			err := in.Validate()

			switch tc.contains {
			case "":
				must.NoError(t, err)
			default:
				must.ErrorContains(t, err, tc.contains)

				// Log error to make it easy to double check output.
				t.Logf("Validate(%s) -> %s", tc.name, err)
			}
		})
	}
}

func TestCSITopology_Contains(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		this     *CSITopology
		other    *CSITopology
		expected bool
	}{
		{
			name: "AWS EBS pre 1.27 behavior",
			this: &CSITopology{
				Segments: map[string]string{
					"topology.ebs.csi.aws.com/zone": "us-east-1a",
				},
			},
			other: &CSITopology{
				Segments: map[string]string{
					"topology.ebs.csi.aws.com/zone": "us-east-1a",
				},
			},
			expected: true,
		},
		{
			name: "AWS EBS post 1.27 behavior",
			this: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone":   "us-east-1a",
					"topology.ebs.csi.aws.com/zone": "us-east-1a",
					"kubernetes.io/os":              "linux",
				},
			},
			other: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone": "us-east-1a",
				},
			},
			expected: true,
		},
		{
			name: "other contains invalid segment value for matched key",
			this: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone":   "us-east-1a",
					"topology.ebs.csi.aws.com/zone": "us-east-1a",
					"kubernetes.io/os":              "linux",
				},
			},
			other: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone": "us-east-1a",
					"kubernetes.io/os":            "windows",
				},
			},
			expected: false,
		},
		{
			name: "other contains invalid segment key",
			this: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone": "us-east-1a",
				},
			},
			other: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone": "us-east-1a",
					"kubernetes.io/os":            "linux",
				},
			},
			expected: false,
		},
		{
			name: "other is nil",
			this: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone": "us-east-1a",
				},
			},
			other:    nil,
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expected, tc.this.Contains(tc.other))
		})
	}

}

func TestNodeRegisterRequest_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name          string
		request       *NodeRegisterRequest
		errorContains string
	}{
		{
			name: "valid",
			request: &NodeRegisterRequest{
				Node: &Node{
					ID:         "node-id",
					SecretID:   "node-secret-id",
					Name:       "node-name",
					NodePool:   "node-pool",
					NodeClass:  "node-class",
					Datacenter: "node-datacenter",
					Attributes: map[string]string{"key1": "value1"},
				},
			},
			errorContains: "",
		},
		{
			name: "nil node",
			request: &NodeRegisterRequest{
				Node: nil,
			},
			errorContains: "missing node for client registration",
		},
		{
			name: "missing ID",
			request: &NodeRegisterRequest{
				Node: &Node{
					ID:         "",
					SecretID:   "node-secret-id",
					Name:       "node-name",
					NodePool:   "node-pool",
					NodeClass:  "node-class",
					Datacenter: "node-datacenter",
					Attributes: map[string]string{"key1": "value1"},
				},
			},
			errorContains: "missing node ID for client registration",
		},
		{
			name: "missing datacenter",
			request: &NodeRegisterRequest{
				Node: &Node{
					ID:         "node-id",
					SecretID:   "node-secret-id",
					Name:       "node-name",
					NodePool:   "node-pool",
					NodeClass:  "node-class",
					Datacenter: "",
					Attributes: map[string]string{"key1": "value1"},
				},
			},
			errorContains: "missing datacenter for client registration",
		},
		{
			name: "missing name",
			request: &NodeRegisterRequest{
				Node: &Node{
					ID:         "node-id",
					SecretID:   "node-secret-id",
					Name:       "",
					NodePool:   "node-pool",
					NodeClass:  "node-class",
					Datacenter: "node-datacenter",
					Attributes: map[string]string{"key1": "value1"},
				},
			},
			errorContains: "missing node name for client registration",
		},
		{
			name: "missing attributes",
			request: &NodeRegisterRequest{
				Node: &Node{
					ID:         "node-id",
					SecretID:   "node-secret-id",
					Name:       "node-name",
					NodePool:   "node-pool",
					NodeClass:  "node-class",
					Datacenter: "node-datacenter",
					Attributes: map[string]string{},
				},
			},
			errorContains: "missing attributes for client registration",
		},
		{
			name: "missing secret ID",
			request: &NodeRegisterRequest{
				Node: &Node{
					ID:         "node-id",
					SecretID:   "",
					Name:       "node-name",
					NodePool:   "node-pool",
					NodeClass:  "node-class",
					Datacenter: "node-datacenter",
					Attributes: map[string]string{"key1": "value1"},
				},
			},
			errorContains: "missing node secret ID for client registration",
		},
		{
			name: "invalid node pool name",
			request: &NodeRegisterRequest{
				Node: &Node{
					ID:         "node-id",
					SecretID:   "node-secret-id",
					Name:       "node-name",
					NodePool:   "****",
					NodeClass:  "node-class",
					Datacenter: "node-datacenter",
					Attributes: map[string]string{"key1": "value1"},
				},
			},
			errorContains: `invalid node pool: invalid name "****"`,
		},
		{
			name: "invalid node pool all use",
			request: &NodeRegisterRequest{
				Node: &Node{
					ID:         "node-id",
					SecretID:   "node-secret-id",
					Name:       "node-name",
					NodePool:   NodePoolAll,
					NodeClass:  "node-class",
					Datacenter: "node-datacenter",
					Attributes: map[string]string{"key1": "value1"},
				},
			},
			errorContains: `node is not allowed to register in node pool "all"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualError := tc.request.Validate()
			if tc.errorContains != "" {
				must.ErrorContains(t, actualError, tc.errorContains)
			} else {
				must.NoError(t, actualError)
			}
		})
	}
}
