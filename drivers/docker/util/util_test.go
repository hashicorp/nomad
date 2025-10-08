// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package util

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestCalculateCPUPercent(t *testing.T) {
	tests := []struct {
		name      string
		newSample uint64
		oldSample uint64
		newTotal  uint64
		oldTotal  uint64
		cores     int
		expected  float64
	}{
		{
			name:      "valid case",
			newSample: 100,
			oldSample: 50,
			newTotal:  200,
			oldTotal:  100,
			cores:     4,
			expected:  200.0,
		},
		{
			name:      "zero denominator",
			newSample: 100,
			oldSample: 50,
			newTotal:  100,
			oldTotal:  100,
			cores:     4,
			expected:  0.0,
		},
		{
			name:      "zero numerator",
			newSample: 100,
			oldSample: 100,
			newTotal:  100,
			oldTotal:  50,
			cores:     4,
			expected:  0.0,
		},
		{
			name:      "negative case",
			newSample: 50,
			oldSample: 100,
			newTotal:  100,
			oldTotal:  200,
			cores:     4,
			expected:  0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateCPUPercent(tt.newSample, tt.oldSample, tt.newTotal, tt.oldTotal, tt.cores)
			must.Eq(t, tt.expected, result)
		})
	}
}
