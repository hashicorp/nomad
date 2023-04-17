// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package jobspec

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

// TestFlattenMapSlice asserts flattenMapSlice recursively flattens a slice of maps into a
// single map.
func TestFlattenMapSlice(t *testing.T) {
	ci.Parallel(t)

	input := map[string]interface{}{
		"foo": 123,
		"bar": []map[string]interface{}{
			{
				"baz": 456,
			},
			{
				"baz": 789,
			},
			{
				"baax": true,
			},
		},
		"nil": nil,
	}

	output := map[string]interface{}{
		"foo": 123,
		"bar": map[string]interface{}{
			"baz":  789,
			"baax": true,
		},
		"nil": nil,
	}

	require.Equal(t, output, flattenMapSlice(input))

}
