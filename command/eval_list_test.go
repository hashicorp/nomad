// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/assert"
)

func TestEvalList_ArgsWithoutPageToken(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		cli      string
		expected string
	}{
		{
			cli:      "nomad eval list -page-token=abcdef",
			expected: "nomad eval list",
		},
		{
			cli:      "nomad eval list -page-token abcdef",
			expected: "nomad eval list",
		},
		{
			cli:      "nomad eval list -per-page 3 -page-token abcdef",
			expected: "nomad eval list -per-page 3",
		},
		{
			cli:      "nomad eval list -page-token abcdef -per-page 3",
			expected: "nomad eval list -per-page 3",
		},
		{
			cli:      "nomad eval list -per-page=3 -page-token abcdef",
			expected: "nomad eval list -per-page=3",
		},
		{
			cli:      "nomad eval list -verbose -page-token abcdef",
			expected: "nomad eval list -verbose",
		},
		{
			cli:      "nomad eval list -page-token abcdef -verbose",
			expected: "nomad eval list -verbose",
		},
		{
			cli:      "nomad eval list -verbose -page-token abcdef -per-page 3",
			expected: "nomad eval list -verbose -per-page 3",
		},
		{
			cli:      "nomad eval list -page-token abcdef -verbose -per-page 3",
			expected: "nomad eval list -verbose -per-page 3",
		},
	}

	for _, tc := range cases {
		args := strings.Split(tc.cli, " ")
		assert.Equal(t, tc.expected, argsWithoutPageToken(args),
			"for input: %s", tc.cli)
	}

}
