// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestMergeMultierrorWarnings(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		errs     []error
		expected string
	}{
		{
			name:     "no warning",
			errs:     []error{},
			expected: "",
		},
		{
			name: "single warning",
			errs: []error{
				errors.New("warning"),
			},
			expected: `
1 warning:

* warning`,
		},
		{
			name: "multiple warnings",
			errs: []error{
				errors.New("warning 1"),
				errors.New("warning 2"),
			},
			expected: `
2 warnings:

* warning 1
* warning 2`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := MergeMultierrorWarnings(tc.errs...)
			must.Eq(t, got, strings.TrimSpace(tc.expected))
		})
	}
}
