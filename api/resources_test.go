// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestResources_Canonicalize(t *testing.T) {
	testutil.Parallel(t)
	testCases := []struct {
		name     string
		input    *Resources
		expected *Resources
	}{
		{
			name:     "empty",
			input:    &Resources{},
			expected: DefaultResources(),
		},
		{
			name: "cores",
			input: &Resources{
				Cores:    pointerOf(2),
				MemoryMB: pointerOf(1024),
			},
			expected: &Resources{
				CPU:      pointerOf(0),
				Cores:    pointerOf(2),
				MemoryMB: pointerOf(1024),
			},
		},
		{
			name: "cpu",
			input: &Resources{
				CPU:      pointerOf(500),
				MemoryMB: pointerOf(1024),
			},
			expected: &Resources{
				CPU:      pointerOf(500),
				Cores:    pointerOf(0),
				MemoryMB: pointerOf(1024),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.input.Canonicalize()
			must.Eq(t, tc.expected, tc.input)
		})
	}
}

func TestResources_Merge(t *testing.T) {
	testutil.Parallel(t)

	none := &NUMAResource{Affinity: "none"}
	prefer := &NUMAResource{Affinity: "prefer"}

	cases := []struct {
		name     string
		resource *Resources
		other    *Resources
		exp      *Resources
	}{
		{
			name:     "merge nil numa",
			resource: &Resources{NUMA: none},
			other:    &Resources{NUMA: nil},
			exp:      &Resources{NUMA: none},
		},
		{
			name:     "merge non-nil numa",
			resource: &Resources{NUMA: none},
			other:    &Resources{NUMA: prefer},
			exp:      &Resources{NUMA: prefer},
		},
	}

	for _, tc := range cases {
		tc.resource.Merge(tc.other)
		must.Eq(t, tc.exp, tc.resource)
	}
}

func TestNUMAResource_Copy(t *testing.T) {
	testutil.Parallel(t)

	r1 := &NUMAResource{Affinity: "none"}
	r2 := r1.Copy()
	r1.Affinity = "require"
	must.Eq(t, "require", r1.Affinity)
	must.Eq(t, "none", r2.Affinity)
}

func TestNUMAResource_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	var n1 *NUMAResource
	n1.Canonicalize()
	must.Nil(t, n1)

	var n2 = &NUMAResource{Affinity: ""}
	n2.Canonicalize()
	must.Eq(t, &NUMAResource{Affinity: "none"}, n2)
}
