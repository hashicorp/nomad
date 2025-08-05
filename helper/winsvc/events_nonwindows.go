// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package winsvc

// SendEvent sends an event to the Windows eventlog
func SendEvent(e Event) {}
