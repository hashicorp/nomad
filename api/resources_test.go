// Copyright IBM Corp. 2015, 2025
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
	r1.Devices = []string{"nvidia/gpu"}
	must.Eq(t, "require", r1.Affinity)
	must.Eq(t, "none", r2.Affinity)
	must.Eq(t, []string{"nvidia/gpu"}, r1.Devices)
	must.SliceEmpty(t, r2.Devices)
}

func TestNUMAResource_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	var n1 *NUMAResource
	n1.Canonicalize()
	must.Nil(t, n1)

	var n2 = &NUMAResource{Affinity: ""}
	n2.Canonicalize()
	must.Eq(t, &NUMAResource{Affinity: "none"}, n2)

	var n3 = &NUMAResource{Affinity: "require", Devices: []string{}}
	n3.Canonicalize()
	must.Eq(t, &NUMAResource{Affinity: "require", Devices: nil}, n3)
}

func TestDeviceOption_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	// Nil option
	var opt *DeviceOption
	opt.Canonicalize() // should not panic

	// Count defaults to 1
	opt2 := &DeviceOption{}
	opt2.Canonicalize()
	must.Eq(t, uint64(1), *opt2.Count)

	// Explicit count preserved
	opt3 := &DeviceOption{Count: pointerOf(uint64(4))}
	opt3.Canonicalize()
	must.Eq(t, uint64(4), *opt3.Count)
}

func TestRequestedDevice_Canonicalize_FirstAvailable(t *testing.T) {
	testutil.Parallel(t)

	// With FirstAvailable, Count should NOT be set to default
	rd := &RequestedDevice{
		Name: "nvidia/gpu",
		FirstAvailable: []*DeviceOption{
			{Count: pointerOf(uint64(2))},
			{}, // no count set
		},
	}
	rd.Canonicalize()

	// Count should remain nil when using FirstAvailable
	must.Nil(t, rd.Count)

	// FirstAvailable options should be canonicalized
	must.Eq(t, uint64(2), *rd.FirstAvailable[0].Count)
	must.Eq(t, uint64(1), *rd.FirstAvailable[1].Count) // defaulted to 1

	// Without FirstAvailable, Count defaults to 1
	rd2 := &RequestedDevice{
		Name: "nvidia/gpu",
	}
	rd2.Canonicalize()
	must.Eq(t, uint64(1), *rd2.Count)
}
