// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"errors"
)

var (
	// ErrVariableLockNotFound is returned when trying to read a lock that
	// does not exist.
	ErrVariableLockNotFound = errors.New("lock not found")
)

// Locks is used to access socks.
type Locks struct {
	vars *Variables
}

// Locks returns a new handle on the locks.
func (c *Client) Locks() *Locks {
	return &Locks{
		vars: &Variables{client: c},
	}
}
