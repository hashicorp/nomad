// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"time"
)

// ExpiryToRenewTime calculates how long until clients should try to renew
// credentials based on their expiration time and now.
//
// Renewals will begin halfway between now and the expiry plus some jitter.
//
// If the expiration is in the past or less than the min wait, then the min
// wait time will be used with jitter.
func ExpiryToRenewTime(exp time.Time, now func() time.Time, minWait time.Duration) time.Duration {
	left := exp.Sub(now())

	renewAt := left / 2

	if renewAt < minWait {
		return minWait + RandomStagger(minWait/10)
	}

	return renewAt + RandomStagger(renewAt/10)
}
