// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"time"
)

func Backoff(backoffBase time.Duration, backoffLimit time.Duration, attempt uint64) time.Duration {
	const MaxUint = ^uint64(0)
	const MaxInt = int64(MaxUint >> 1)

	// Ensure lack of non-positive backoffs since these make no sense
	if backoffBase.Nanoseconds() <= 0 {
		return max(backoffBase, 0*time.Second)
	}

	// Ensure that a large attempt will not cause an overflow
	if attempt > 62 || MaxInt/backoffBase.Nanoseconds() < (1<<attempt) {
		return backoffLimit
	}

	// Compute deadline and clamp it to backoffLimit
	deadline := 1 << attempt * backoffBase
	if deadline > backoffLimit {
		deadline = backoffLimit
	}

	return deadline
}
