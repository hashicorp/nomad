package api

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestWorkloadIdentity_Canonicalize(t *testing.T) {
	testCases := []struct {
		name     string
		input    *WorkloadIdentity
		expected *WorkloadIdentity
	}{
		{
			name:     "nil",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty",
			input: &WorkloadIdentity{},
			expected: &WorkloadIdentity{
				Env:  pointerOf(true),
				File: pointerOf(true),
			},
		},
		{
			name: "env-false",
			input: &WorkloadIdentity{
				Env: pointerOf(false),
			},
			expected: &WorkloadIdentity{
				Env:  pointerOf(false),
				File: pointerOf(true),
			},
		},
		{
			name: "both-false",
			input: &WorkloadIdentity{
				Env:  pointerOf(false),
				File: pointerOf(false),
			},
			expected: &WorkloadIdentity{
				Env:  pointerOf(false),
				File: pointerOf(false),
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
