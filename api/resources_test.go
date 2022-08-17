package api

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/kr/pretty"
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
				Cores:    pointer.Of(2),
				MemoryMB: pointer.Of(1024),
			},
			expected: &Resources{
				CPU:      pointer.Of(0),
				Cores:    pointer.Of(2),
				MemoryMB: pointer.Of(1024),
			},
		},
		{
			name: "cpu",
			input: &Resources{
				CPU:      pointer.Of(500),
				MemoryMB: pointer.Of(1024),
			},
			expected: &Resources{
				CPU:      pointer.Of(500),
				Cores:    pointer.Of(0),
				MemoryMB: pointer.Of(1024),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.input.Canonicalize()
			if !reflect.DeepEqual(tc.input, tc.expected) {
				t.Fatalf("Name: %v, Diffs:\n%v", tc.name, pretty.Diff(tc.expected, tc.input))
			}
		})
	}
}
