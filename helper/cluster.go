// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"math/rand"
	"time"
)

const (
	// minRate is the minimum rate at which we allow an action to be performed
	// across the whole cluster. The value is once a day: 1 / (1 * time.Day)
	minRate = 1.0 / 86400
)

// RandomStagger returns an interval between 0 and the duration
func RandomStagger(interval time.Duration) time.Duration {
	if interval <= 0 {
		return 0
	}
	return time.Duration(uint64(rand.Int63()) % uint64(interval))
}

// RateScaledInterval is used to choose an interval to perform an action in
// order to target an aggregate number of actions per second across the whole
// cluster.
func RateScaledInterval(rate float64, min time.Duration, n int) time.Duration {
	if rate <= minRate {
		return min
	}
	interval := time.Duration(float64(time.Second) * float64(n) / rate)
	if interval < min {
		return min
	}

	return interval
}
