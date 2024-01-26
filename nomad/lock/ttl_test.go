// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lock

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestTimer(t *testing.T) {
	ci.Parallel(t)

	// Create a test channel and timer test function that will be used
	// throughout the test.
	newTimerCh := make(chan int)

	waitForTimer := func() {
		select {
		case <-newTimerCh:
			return
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timer did not fire")
		}
	}

	timer := NewTTLTimer()

	// Perform a lookup on a timer that doesn't exist, to ensure this is
	// handled properly.
	must.Nil(t, timer.Get("this-does-not-exist"))

	// Perform a create, read, update, and delete on a single timer.
	timer.Create("test-timer-2", time.Millisecond, func() { newTimerCh <- 1 })
	must.Eq(t, 1, timer.TimerNum())
	waitForTimer()

	// Ensure the timer is still held within the mapping.
	must.Eq(t, 1, timer.TimerNum())

	// Update the timer and check that it fires again.
	timer.Create("test-timer-2", time.Millisecond, nil)
	waitForTimer()

	// Reset the timer with a long ttl and then stop it.
	timer.Create("test-timer-2", 20*time.Millisecond, func() { newTimerCh <- 1 })
	timer.StopAndRemove("test-timer-2")

	select {
	case <-newTimerCh:
		t.Fatal("timer fired although it shouldn't")
	case <-time.After(100 * time.Millisecond):
	}

	// Ensure that stopping a stopped timer does not break anything.
	timer.StopAndRemove("test-timer-2")
	must.Eq(t, 0, timer.TimerNum())

	// Create two timers, stopping them using the StopAll function to signify
	// leadership loss.
	timer.Create("test-timer-3", 20*time.Millisecond, func() { newTimerCh <- 1 })
	timer.Create("test-timer-4", 30*time.Millisecond, func() { newTimerCh <- 2 })
	timer.StopAndRemoveAll()

	select {
	case msg := <-newTimerCh:
		t.Fatalf("timer %d fired although it shouldn't", msg)
	case <-time.After(100 * time.Millisecond):
	}

	must.Eq(t, 0, timer.TimerNum())
}
