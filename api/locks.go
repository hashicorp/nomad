// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"context"
	"fmt"
	"time"
)

type lock interface {
	Acquire(ctx context.Context, callerID string) (string, error)
	Release(ctx context.Context) error
	Renew(ctx context.Context) error
}

const (
	// DefaultLockSessionTTL is the default session TTL if no Session is provided
	// when creating a new Lock. This is used because we do not have another
	// other check to depend upon.
	DefaultLockTTL = 15 * time.Second

	// DefaultLockWaitTime is how long we block for at a time to check if lock
	// acquisition is possible. This affects the minimum time it takes to cancel
	// a Lock acquisition.
	DefaultLockDelay = 15 * time.Second
)

var (
	// ErrLockHeld is returned if we attempt to double lock
	ErrLockHeld = fmt.Errorf("Lock already held")

	// ErrLockNotHeld is returned if we attempt to unlock a lock
	// that we do not hold.
	ErrLockNotHeld = fmt.Errorf("Lock not held")

	// ErrLockInUse is returned if we attempt to destroy a lock
	// that is in use.
	ErrLockInUse = fmt.Errorf("Lock in use")

	// ErrLockConflict is returned if the flags on a key
	// used for a lock do not match expectation
	ErrLockConflict = fmt.Errorf("Existing key does not match lock use")
)

// Lock is used to implement client-side leader election. It is follows the
// algorithm as described here: https://www.consul.io/docs/guides/leader-election.html.
type Locks struct {
	c *Client
}

// LockOpts returns a handle to a lock struct which can be used
// to acquire and release the mutex. The key used must have
// write permissions.
func (c *Client) Locks() (*Locks, error) {
	l := &Locks{
		c: c,
	}

	return l, nil
}
