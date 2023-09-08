// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
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
				Deadline:         pointer.Of("5m"),
				IgnoreSystemJobs: nil,
				Force:            nil,
			},
			expectedOutput: &DrainConfig{
				Deadline:         pointer.Of("5m"),
				IgnoreSystemJobs: nil,
				Force:            nil,
			},
		},
		{
			name: "full config",
			inputDrainConfig: &DrainConfig{
				Deadline:         pointer.Of("5m"),
				IgnoreSystemJobs: pointer.Of(false),
				Force:            pointer.Of(true),
			},
			expectedOutput: &DrainConfig{
				Deadline:         pointer.Of("5m"),
				IgnoreSystemJobs: pointer.Of(false),
				Force:            pointer.Of(true),
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
				Deadline:         pointer.Of("5m"),
				IgnoreSystemJobs: pointer.Of(false),
				Force:            pointer.Of(true),
			},
			expectedOutput: &DrainConfig{
				Deadline:         pointer.Of("5m"),
				IgnoreSystemJobs: pointer.Of(false),
				Force:            pointer.Of(true),
			},
		},
		{
			name: "nil merge",
			inputDrainConfig: &DrainConfig{
				Deadline:         pointer.Of("5m"),
				IgnoreSystemJobs: pointer.Of(false),
				Force:            pointer.Of(true),
			},
			mergeDrainConfig: nil,
			expectedOutput: &DrainConfig{
				Deadline:         pointer.Of("5m"),
				IgnoreSystemJobs: pointer.Of(false),
				Force:            pointer.Of(true),
			},
		},
		{
			name: "partial",
			inputDrainConfig: &DrainConfig{
				Deadline:         pointer.Of("5m"),
				IgnoreSystemJobs: pointer.Of(false),
				Force:            nil,
			},
			mergeDrainConfig: &DrainConfig{
				Deadline:         nil,
				IgnoreSystemJobs: nil,
				Force:            pointer.Of(true),
			},
			expectedOutput: &DrainConfig{
				Deadline:         pointer.Of("5m"),
				IgnoreSystemJobs: pointer.Of(false),
				Force:            pointer.Of(true),
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
