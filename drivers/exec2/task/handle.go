// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package task

import "sync"

type Handle struct {
	lock sync.RWMutex

	// YOU ARE HERE; copy more from pledge
}
