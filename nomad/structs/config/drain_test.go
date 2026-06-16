// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/v2/ci"
	"github.com/shoenig/test/must"
)

func TestDrainConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name             string
		inputDrainConfig *DrainConfig
		expectedOutput   *DrainConfig
	}{
		{
			name:             "nil config",
			inputDrainConfig: nil,
			expectedOutput:   nil,
		},
		{
			name: "partial config",
			inputDrainConfig: &DrainConfig{
				Deadline:         new("5m"),
				IgnoreSystemJobs: nil,
				Force:            nil,
			},
			expectedOutput: &DrainConfig{
				Deadline:         new("5m"),
				IgnoreSystemJobs: nil,
				Force:            nil,
			},
		},
		{
			name: "full config",
			inputDrainConfig: &DrainConfig{
				Deadline:         new("5m"),
				IgnoreSystemJobs: new(false),
				Force:            new(true),
			},
			expectedOutput: &DrainConfig{
				Deadline:         new("5m"),
				IgnoreSystemJobs: new(false),
				Force:            new(true),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputDrainConfig.Copy()
			must.Eq(t, tc.expectedOutput, actualOutput)

			if tc.inputDrainConfig != nil {
				must.NotEq(t, fmt.Sprintf("%p", tc.inputDrainConfig), fmt.Sprintf("%p", actualOutput))
			}
		})
	}
}

func TestDrainConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name             string
		inputDrainConfig *DrainConfig
		mergeDrainConfig *DrainConfig
		expectedOutput   *DrainConfig
	}{
		{
			name:             "nil",
			inputDrainConfig: nil,
			mergeDrainConfig: nil,
			expectedOutput:   nil,
		},
		{
			name:             "nil input",
			inputDrainConfig: nil,
			mergeDrainConfig: &DrainConfig{
				Deadline:         new("5m"),
				IgnoreSystemJobs: new(false),
				Force:            new(true),
			},
			expectedOutput: &DrainConfig{
				Deadline:         new("5m"),
				IgnoreSystemJobs: new(false),
				Force:            new(true),
			},
		},
		{
			name: "nil merge",
			inputDrainConfig: &DrainConfig{
				Deadline:         new("5m"),
				IgnoreSystemJobs: new(false),
				Force:            new(true),
			},
			mergeDrainConfig: nil,
			expectedOutput: &DrainConfig{
				Deadline:         new("5m"),
				IgnoreSystemJobs: new(false),
				Force:            new(true),
			},
		},
		{
			name: "partial",
			inputDrainConfig: &DrainConfig{
				Deadline:         new("5m"),
				IgnoreSystemJobs: new(false),
				Force:            nil,
			},
			mergeDrainConfig: &DrainConfig{
				Deadline:         nil,
				IgnoreSystemJobs: nil,
				Force:            new(true),
			},
			expectedOutput: &DrainConfig{
				Deadline:         new("5m"),
				IgnoreSystemJobs: new(false),
				Force:            new(true),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputDrainConfig.Merge(tc.mergeDrainConfig)
			must.Eq(t, tc.expectedOutput, actualOutput)
		})
	}
}
