// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper"
)

var chanEvents = make(chan Event)

// SendEvent sends an event to the Windows eventlog
func SendEvent(e Event) {
	timer, stop := helper.NewSafeTimer(100 * time.Millisecond)
	defer stop()

	select {
	case chanEvents <- e:
	case <-timer.C:
		hclog.L().Error("failed to send event to windows eventlog, timed out",
			"event", e)
	}
}
