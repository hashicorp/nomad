// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func Test_isLogLevelValid(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name           string
		inputLevel     string
		expectedOutput bool
	}{
		{
			name:           "trace",
			inputLevel:     "TRACE",
			expectedOutput: true,
		},
		{
			name:           "debug",
			inputLevel:     "DEBUG",
			expectedOutput: true,
		},
		{
			name:           "info",
			inputLevel:     "INFO",
			expectedOutput: true,
		},
		{
			name:           "warn",
			inputLevel:     "WARN",
			expectedOutput: true,
		},
		{
			name:           "error",
			inputLevel:     "ERROR",
			expectedOutput: true,
		},
		{
			name:           "off",
			inputLevel:     "OFF",
			expectedOutput: true,
		},
		{
			name:           "invalid",
			inputLevel:     "INVALID",
			expectedOutput: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expectedOutput, isLogLevelValid(tc.inputLevel))
		})
	}
}
