// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
)

func TestWorkloadIdentityConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	original := &WorkloadIdentityConfig{
		Name:     "test",
		Audience: []string{"aud"},
		Env:      pointer.Of(true),
		File:     pointer.Of(false),
	}

	// Verify Copy() returns the same values but different pointer.
	clone := original.Copy()
	must.Eq(t, original, clone)
	must.NotEqOp(t, original, clone)

	// Verify returned struct does not mutate original.
	clone.Name = "clone"
	clone.Audience = []string{"aud", "clone"}
	clone.Env = pointer.Of(false)
	clone.File = pointer.Of(true)

	must.NotEq(t, original, clone)
	must.NotEqOp(t, original, clone)
}

func TestWorkloadIdentityConfig_Equal(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		a        *WorkloadIdentityConfig
		b        *WorkloadIdentityConfig
		expectEq bool
	}{
		{
			name: "equal",
			a: &WorkloadIdentityConfig{
				Name:     "test",
				Audience: []string{"aud"},
				Env:      pointer.Of(true),
				File:     pointer.Of(false),
			},
			b: &WorkloadIdentityConfig{
				Name:     "test",
				Audience: []string{"aud"},
				Env:      pointer.Of(true),
				File:     pointer.Of(false),
			},
			expectEq: true,
		},
		{
			name: "different name",
			a: &WorkloadIdentityConfig{
				Name: "a",
			},
			b: &WorkloadIdentityConfig{
				Name: "b",
			},
			expectEq: false,
		},
		{
			name: "different audience",
			a: &WorkloadIdentityConfig{
				Audience: []string{"a"},
			},
			b: &WorkloadIdentityConfig{
				Audience: []string{"b"},
			},
			expectEq: false,
		},
		{
			name: "different env",
			a: &WorkloadIdentityConfig{
				Env: pointer.Of(true),
			},
			b: &WorkloadIdentityConfig{
				Env: pointer.Of(false),
			},
			expectEq: false,
		},
		{
			name: "different env nil",
			a: &WorkloadIdentityConfig{
				Env: pointer.Of(true),
			},
			b: &WorkloadIdentityConfig{
				Env: nil,
			},
			expectEq: false,
		},
		{
			name: "different file",
			a: &WorkloadIdentityConfig{
				File: pointer.Of(true),
			},
			b: &WorkloadIdentityConfig{
				File: pointer.Of(false),
			},
			expectEq: false,
		},
		{
			name: "different file nil",
			a: &WorkloadIdentityConfig{
				File: pointer.Of(true),
			},
			b: &WorkloadIdentityConfig{
				File: nil,
			},
			expectEq: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectEq {
				must.True(t, tc.a.Equal(tc.b))
			} else {
				must.False(t, tc.a.Equal(tc.b))
			}
		})
	}
}

func TestWorkloadIdentityConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		other    *WorkloadIdentityConfig
		expected *WorkloadIdentityConfig
	}{
		{
			name: "merge name",
			other: &WorkloadIdentityConfig{
				Name: "other",
			},
			expected: &WorkloadIdentityConfig{
				Name:     "other",
				Audience: []string{"aud"},
				Env:      pointer.Of(true),
				File:     pointer.Of(false),
			},
		},
		{
			name: "merge audience",
			other: &WorkloadIdentityConfig{
				Audience: []string{"aud", "other"},
			},
			expected: &WorkloadIdentityConfig{
				Name:     "test",
				Audience: []string{"aud", "other"},
				Env:      pointer.Of(true),
				File:     pointer.Of(false),
			},
		},
		{
			name: "merge env",
			other: &WorkloadIdentityConfig{
				Env: pointer.Of(false),
			},
			expected: &WorkloadIdentityConfig{
				Name:     "test",
				Audience: []string{"aud"},
				Env:      pointer.Of(false),
				File:     pointer.Of(false),
			},
		},
		{
			name: "merge file",
			other: &WorkloadIdentityConfig{
				File: pointer.Of(true),
			},
			expected: &WorkloadIdentityConfig{
				Name:     "test",
				Audience: []string{"aud"},
				Env:      pointer.Of(true),
				File:     pointer.Of(true),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := &WorkloadIdentityConfig{
				Name:     "test",
				Audience: []string{"aud"},
				Env:      pointer.Of(true),
				File:     pointer.Of(false),
			}
			got := original.Merge(tc.other)
			must.Eq(t, tc.expected, got)
		})
	}
}
