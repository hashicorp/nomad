// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lock

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestDelay(t *testing.T) {
	ci.Parallel(t)

	delay := NewDelayTimer()

	// An unknown key should have a time in the past.
	must.True(t, delay.Get("this-does-not-exist").Before(time.Now()))
	must.Eq(t, 0, delay.timerNum())

	// Add a key and set a short expiration.
	timeNow := time.Now()
	delay.Set("test-delay-1", timeNow, 100*time.Millisecond)
	must.False(t, delay.Get("test-delay-1").Before(time.Now()))
	must.Eq(t, 1, delay.timerNum())

	// Wait for the key to expire and check again.
	time.Sleep(120 * time.Millisecond)
	must.True(t, delay.Get("test-delay-1").Before(timeNow))
	must.Eq(t, 0, delay.timerNum())

	// Add a key and set a long expiration.
	timeNow = time.Now()
	delay.Set("test-delay-2", timeNow, 1000*time.Millisecond)
	must.False(t, delay.Get("test-delay-2").Before(time.Now()))
	must.Eq(t, 1, delay.timerNum())

	// Perform the stop call which the leader will do when stepping down.
	delay.RemoveAll()
	must.Eq(t, 0, delay.timerNum())
}
