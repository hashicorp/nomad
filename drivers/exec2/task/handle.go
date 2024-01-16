// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package task

import "sync"

type Handle struct {
	lock sync.RWMutex
}
