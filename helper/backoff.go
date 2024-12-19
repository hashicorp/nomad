// Copyright (c) HashiCorp, Inc.
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
	deadline := 1 << attempt * backoffBase
	if deadline > backoffLimit {
		deadline = backoffLimit
	}

	return deadline
}

// WithBackoffFunc is a helper that runs a function with geometric backoff + a
// small jitter to a maximum backoff. It returns once the context closes, with
// the error wrapping over the error from the function.
func WithBackoffFunc(ctx context.Context, minBackoff, maxBackoff time.Duration, fn func() error) error {
	var err error
	backoff := minBackoff
	t, stop := NewSafeTimer(0)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled: %w", err)
		case <-t.C:
		}

		err = fn()
		if err == nil {
			return nil
		}

		if backoff < maxBackoff {
			backoff = backoff*2 + RandomStagger(minBackoff/10)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		t.Reset(backoff)
	}
}
