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
