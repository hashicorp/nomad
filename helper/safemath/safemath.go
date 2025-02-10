// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package safemath

import "math"

// Add adds integers but clamps the results to MaxInt64 if there's overflow
func Add(a, b int64) int64 {
	c := a + b
	if (c > a) == (b > 0) {
		return c
	}
	return math.MaxInt64
}
