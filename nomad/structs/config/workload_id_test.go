// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"
	"time"

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
		Filepath: "foo",
		TTL:      pointer.Of(time.Hour),
	}

	// Verify Copy() returns the same values but different pointer.
	clone := original.Copy()
	must.Eq(t, original, clone)
	must.NotEqOp(t, original, clone)
	must.NotEqOp(t, original.Env, clone.Env)
	must.NotEqOp(t, original.File, clone.File)
	must.NotEqOp(t, original.TTL, clone.TTL)

	// Verify returned struct does not mutate original.
	clone.Name = "clone"
	clone.Audience[0] = "changed"
	*clone.Env = false
	*clone.File = true
	clone.Filepath = "changed"
	*clone.TTL = time.Second

	must.NotEq(t, original, clone)
	must.NotEqOp(t, original, clone)
	must.NotEqOp(t, original.Env, clone.Env)
	must.NotEqOp(t, original.File, clone.File)
	must.NotEqOp(t, original.Filepath, clone.Filepath)
	must.NotEqOp(t, original.TTL, clone.TTL)
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
				Filepath: "foo",
				TTL:      pointer.Of(time.Hour),
			},
			b: &WorkloadIdentityConfig{
				Name:     "test",
				Audience: []string{"aud"},
				Env:      pointer.Of(true),
				File:     pointer.Of(false),
				Filepath: "foo",
				TTL:      pointer.Of(time.Hour),
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
			name: "different filepath",
			a: &WorkloadIdentityConfig{
				Filepath: "a",
			},
			b: &WorkloadIdentityConfig{
				Filepath: "b",
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
		{
			name: "different ttl",
			a: &WorkloadIdentityConfig{
				TTL: pointer.Of(time.Hour),
			},
			b: &WorkloadIdentityConfig{
				TTL: pointer.Of(time.Minute),
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
				Filepath: "test",
				TTL:      pointer.Of(time.Hour),
			},
		},
		{
			name: "merge audience",
			other: &WorkloadIdentityConfig{
				Audience: []string{"aud", "aud", "other"},
			},
			expected: &WorkloadIdentityConfig{
				Name:     "test",
				Audience: []string{"aud", "other"},
				Env:      pointer.Of(true),
				File:     pointer.Of(false),
				Filepath: "test",
				TTL:      pointer.Of(time.Hour),
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
				Filepath: "test",
				TTL:      pointer.Of(time.Hour),
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
				Filepath: "test",
				TTL:      pointer.Of(time.Hour),
			},
		},
		{
			name: "merge filepath",
			other: &WorkloadIdentityConfig{
				Filepath: "other",
			},
			expected: &WorkloadIdentityConfig{
				Name:     "test",
				Audience: []string{"aud"},
				Env:      pointer.Of(true),
				File:     pointer.Of(false),
				Filepath: "other",
				TTL:      pointer.Of(time.Hour),
			},
		},
		{
			name: "merge ttl",
			other: &WorkloadIdentityConfig{
				TTL: pointer.Of(time.Second),
			},
			expected: &WorkloadIdentityConfig{
				Name:     "test",
				Audience: []string{"aud"},
				Env:      pointer.Of(true),
				File:     pointer.Of(false),
				Filepath: "test",
				TTL:      pointer.Of(time.Second),
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
				Filepath: "test",
				TTL:      pointer.Of(time.Hour),
			}
			got := original.Merge(tc.other)
			must.Eq(t, tc.expected, got)
		})
	}
}

func TestWorkloadIdentityConfig_Merge_multiple(t *testing.T) {
	widConfig1 := &WorkloadIdentityConfig{
		Name:     "wid1",
		Audience: []string{"aud1"},
		Env:      pointer.Of(true),
	}

	widConfig2 := &WorkloadIdentityConfig{
		Name:     "wid2",
		Audience: []string{"aud2"},
		Env:      pointer.Of(false),
	}

	got12 := widConfig1.Merge(widConfig2)
	must.Eq(t, &WorkloadIdentityConfig{
		Name:     "wid2",
		Audience: []string{"aud1", "aud2"},
		Env:      pointer.Of(false),
	}, got12)

	widConfig3 := &WorkloadIdentityConfig{
		Name:     "wid3",
		Audience: []string{"aud1", "aud2", "aud3"},
		File:     pointer.Of(false),
	}

	got123 := got12.Merge(widConfig3)
	must.Eq(t, &WorkloadIdentityConfig{
		Name:     "wid3",
		Audience: []string{"aud1", "aud2", "aud3"},
		Env:      pointer.Of(false),
		File:     pointer.Of(false),
	}, got123)
}
