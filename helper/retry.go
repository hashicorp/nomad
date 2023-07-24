// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package helper

import "time"

// ExpiryToRenewTime calculates how long until clients should try to renew
// credentials based on their expiration time and now.
//
// Renewals will begin halfway between now and the expiry plus some jitter.
func ExpiryToRenewTime(exp time.Time, now func() time.Time, minWait time.Duration) time.Duration {
	left := exp.Sub(now())

	if left < minWait {
		left = minWait
	}

	return (left / 2) + RandomStagger(left/10)
}
