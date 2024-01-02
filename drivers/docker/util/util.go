// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package util

func CalculateCPUPercent(newSample, oldSample, newTotal, oldTotal uint64, cores int) float64 {
	numerator := newSample - oldSample
	denom := newTotal - oldTotal
	if numerator <= 0 || denom <= 0 {
		return 0.0
	}

	return (float64(numerator) / float64(denom)) * float64(cores) * 100.0
}
