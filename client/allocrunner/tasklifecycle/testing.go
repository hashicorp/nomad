// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tasklifecycle

import (
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	testing "github.com/mitchellh/go-testing-interface"
)

func RequireTaskBlocked(t testing.T, c *Coordinator, task *structs.Task) {
	ch := c.StartConditionForTask(task)
	requireChannelBlocking(t, ch, task.Name)
}

func RequireTaskAllowed(t testing.T, c *Coordinator, task *structs.Task) {
	ch := c.StartConditionForTask(task)
	requireChannelPassing(t, ch, task.Name)
}

func WaitNotInitUntil(c *Coordinator, until time.Duration, errorFunc func()) {
	testutil.WaitForResultUntil(until,
		func() (bool, error) {
			c.currentStateLock.RLock()
			defer c.currentStateLock.RUnlock()
			return c.currentState != coordinatorStateInit, nil
		},
		func(_ error) {
			errorFunc()
		})
}

func requireChannelPassing(t testing.T, ch <-chan struct{}, name string) {
	testutil.WaitForResult(func() (bool, error) {
		return !isChannelBlocking(ch), nil
	}, func(_ error) {
		t.Fatalf("%s channel was blocking, should be passing", name)
	})
}

func requireChannelBlocking(t testing.T, ch <-chan struct{}, name string) {
	testutil.WaitForResult(func() (bool, error) {
		return isChannelBlocking(ch), nil
	}, func(_ error) {
		t.Fatalf("%s channel was passing, should be blocking", name)
	})
}

func isChannelBlocking(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return false
	default:
		return true
	}
}
