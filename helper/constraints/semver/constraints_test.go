// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package semver

import (
	"testing"

	"github.com/hashicorp/go-version"
)

// This file is a copy of github.com/hashicorp/go-version/constraint_test.go
// with minimal changes to demonstrate differences. Diffing the files should
// illustrate behavior differences in Constraint and version.Constraint.

func TestNewConstraint(t *testing.T) {
	cases := []struct {
		input string
		count int
		err   bool
	}{
		{">= 1.2", 1, false},
		{"1.0", 1, false},
		{">= 1.x", 0, true},
		{">= 1.2, < 1.0", 2, false},

		// Out of bounds
		{"11387778780781445675529500000000000000000", 0, true},

		// Semver only
		{">= 1.0beta1", 0, true},

		// No pessimistic operator
		{"~> 1.0", 0, true},
	}

	for _, tc := range cases {
		v, err := NewConstraint(tc.input)
		if tc.err && err == nil {
			t.Fatalf("expected error for input: %s", tc.input)
		} else if !tc.err && err != nil {
			t.Fatalf("error for input %s: %s", tc.input, err)
		}

		if len(v) != tc.count {
			t.Fatalf("input: %s\nexpected len: %d\nactual: %d",
				tc.input, tc.count, len(v))
		}
	}
}

func TestConstraintCheck(t *testing.T) {
	cases := []struct {
		constraint string
		version    string
		check      bool
	}{
		{">= 1.0, < 1.2", "1.1.5", true},
		{"< 1.0, < 1.2", "1.1.5", false},
		{"= 1.0", "1.1.5", false},
		{"= 1.0", "1.0.0", true},
		{"1.0", "1.0.0", true},

		// Assert numbers are *not* compared lexically as in #4729
		{"> 10", "8", false},

		// Pre-releases are ordered according to Semver v2
		{"> 2.0", "2.1.0-beta", true},
		{"> 2.1.0-a", "2.1.0-beta", true},
		{"> 2.1.0-a", "2.1.1-beta", true},
		{"> 2.0.0", "2.1.0-beta", true},
		{"> 2.1.0-a", "2.1.1", true},
		{"> 2.1.0-a", "2.1.1-beta", true},
		{"> 2.1.0-a", "2.1.0", true},
		{"<= 2.1.0-a", "2.0.0", true},
		{">= 0.6.1", "1.3.0-beta1", true},
		{"> 1.0-beta1", "1.0-rc1", true},

		// Meta components are ignored according to Semver v2
		{">= 0.6.1", "1.3.0-beta1+ent", true},
		{">= 1.3.0-beta1", "1.3.0-beta1+ent", true},
		{"> 1.3.0-beta1+cgo", "1.3.0-beta1+ent", false},
		{"= 1.3.0-beta1+cgo", "1.3.0-beta1+ent", true},
	}

	for _, tc := range cases {
		c, err := NewConstraint(tc.constraint)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		v, err := version.NewSemver(tc.version)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		actual := c.Check(v)
		expected := tc.check
		if actual != expected {
			t.Fatalf("Version: %s\nConstraint: %s\nExpected: %#v",
				tc.version, tc.constraint, expected)
		}
	}
}

func TestConstraintsString(t *testing.T) {
	cases := []struct {
		constraint string
		result     string
	}{
		{">= 1.0, < 1.2", ""},
	}

	for _, tc := range cases {
		c, err := NewConstraint(tc.constraint)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		actual := c.String()
		expected := tc.result
		if expected == "" {
			expected = tc.constraint
		}

		if actual != expected {
			t.Fatalf("Constraint: %s\nExpected: %#v\nActual: %s",
				tc.constraint, expected, actual)
		}
	}
}
