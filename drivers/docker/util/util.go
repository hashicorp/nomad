// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package util

func CalculateCPUPercent(newSample, oldSample, newTotal, oldTotal uint64, cores int) float64 {
	if (newSample <= oldSample) || (newTotal <= oldTotal) {
		return 0.0
	}
	numerator := newSample - oldSample
	denom := newTotal - oldTotal

	return (float64(numerator) / float64(denom)) * float64(cores) * 100.0
}
