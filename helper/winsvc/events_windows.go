// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

var chanEvents = make(chan Event)

// SendEvent sends an event to the Windows eventlog
func SendEvent(e Event) {
	chanEvents <- e
}
