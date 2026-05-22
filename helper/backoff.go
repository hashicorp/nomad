// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"context"
	"fmt"
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
	deadline := min(1<<attempt*backoffBase, backoffLimit)

	return deadline
}

// WithBackoffFunc is a helper that runs a function with geometric backoff + a
// small jitter to a maximum backoff. It returns once the context closes, with
// the error wrapping over the error from the function.
func WithBackoffFunc(ctx context.Context, minBackoff, maxBackoff time.Duration, fn func() error) error {
	var (
		backoff time.Duration = minBackoff
		err     error
	)

	for {
		if err = fn(); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled: %w", err)
		case <-time.After(backoff):
		}

		if backoff < maxBackoff {
			backoff = min(backoff*2+RandomStagger(minBackoff/10), maxBackoff)
		}
	}
}
