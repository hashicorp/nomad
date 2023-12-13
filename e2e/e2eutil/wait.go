// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package e2eutil

import "time"

// WaitConfig is an interval and wait time that can be passed to a waiter
// function, but with a default value that comes from the OrDefault method
// if the config is nil
type WaitConfig struct {
	Interval time.Duration
	Retries  int64
}

// OrDefault returns a default wait config of 10s.
func (wc *WaitConfig) OrDefault() (time.Duration, int64) {
	if wc == nil {
		return time.Millisecond * 100, 100
	}
	if wc.Interval == 0 {
		wc.Interval = time.Millisecond * 100
	}
	if wc.Retries == 0 {
		wc.Retries = 100
	}
	return wc.Interval, wc.Retries
}
