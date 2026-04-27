// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"time"

	"github.com/hashicorp/go-hclog"
)

var chanEvents = make(chan Event)

// SendEvent sends an event to the Windows eventlog
func SendEvent(e Event) {
	select {
	case chanEvents <- e:
	case <-time.After(100 * time.Millisecond):
		hclog.L().Error("failed to send event to windows eventlog, timed out",
			"event", e)
	}
}
