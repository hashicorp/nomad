package api

import (
	"reflect"
	"testing"

	"github.com/kr/pretty"
)

func TestResources_Canonicalize(t *testing.T) {
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
				Cores:    intToPtr(2),
				MemoryMB: intToPtr(1024),
			},
			expected: &Resources{
				CPU:      intToPtr(0),
				Cores:    intToPtr(2),
				MemoryMB: intToPtr(1024),
			},
		},
		{
			name: "cpu",
			input: &Resources{
				CPU:      intToPtr(500),
				MemoryMB: intToPtr(1024),
			},
			expected: &Resources{
				CPU:      intToPtr(500),
				Cores:    intToPtr(0),
				MemoryMB: intToPtr(1024),
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
