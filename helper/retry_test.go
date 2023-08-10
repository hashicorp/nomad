// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

// TestExpiryToRenewTime_0Min asserts that ExpiryToRenewTime with a 0 min wait
// will cause an immediate renewal
func TestExpiryToRenewTime_0Min(t *testing.T) {
	exp := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	now := func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 1, 0, time.UTC)
	}

	renew := ExpiryToRenewTime(exp, now, 0)

	must.Zero(t, renew)
}

// TestExpiryToRenewTime_14Days asserts that ExpiryToRenewTime begins trying to
// renew at or after 7 days of a 14 day expiration window.
func TestExpiryToRenewTime_30Days(t *testing.T) {
	exp := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
	now := func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	min := 20 * time.Minute

	renew := ExpiryToRenewTime(exp, now, min)

	// Renew should be much greater than min wait
	must.Greater(t, min, renew)

	// Renew should be >= 7 days
	must.GreaterEq(t, 7*24*time.Hour, renew)
}

// TestExpiryToRenewTime_UnderMin asserts that ExpiryToRenewTime uses the min
// wait + jitter if it is greater than the time until expiry.
func TestExpiryToRenewTime_UnderMin(t *testing.T) {
	exp := time.Date(2023, 1, 1, 0, 0, 10, 0, time.UTC)
	now := func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	min := 20 * time.Second

	renew := ExpiryToRenewTime(exp, now, min)

	// Renew should be >= min wait (jitter can be 0)
	must.GreaterEq(t, min, renew)

	// When we fallback to the min wait it means we miss the expiration, but this
	// is necessary to prevent stampedes after outages and partitions.
	must.GreaterEq(t, exp.Sub(now()), renew)
}

// TestExpiryToRenewTime_Expired asserts that ExpiryToRenewTime defaults to
// minWait (+jitter) if the renew time has already elapsed.
func TestExpiryToRenewTime_Expired(t *testing.T) {
	exp := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	now := func() time.Time {
		return time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)
	}
	min := time.Hour

	renew := ExpiryToRenewTime(exp, now, min)

	must.Greater(t, min, renew)
	must.Less(t, min*2, renew)
}
