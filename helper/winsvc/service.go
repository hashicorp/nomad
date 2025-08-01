// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"runtime"
)

const (
	WINDOWS_SERVICE_NAME              = "nomad"
	WINDOWS_SERVICE_DISPLAY_NAME      = "HashiCorp Nomad"
	WINDOWS_SERVICE_DESCRIPTION       = "Workload scheduler and orchestrator - https://nomadproject.io"
	WINDOWS_INSTALL_BIN_DIRECTORY     = `{{.ProgramFiles}}\HashiCorp\nomad\bin`
	WINDOWS_INSTALL_APPDATA_DIRECTORY = `{{.ProgramData}}\HashiCorp\nomad`

	// Number of seconds to wait for a
	// service to reach a desired state
	WINDOWS_SERVICE_STATE_TIMEOUT = 60
)

var chanGraceExit = make(chan int)
var chanEvents = make(chan Event)

// SendEvent sends an event to the Windows eventlog
func SendEvent(e Event) {
	if runtime.GOOS == "windows" {
		chanEvents <- e
	}
}

// ShutdownChannel returns a channel that sends a message that a shutdown
// signal has been received for the service.
func ShutdownChannel() <-chan int {
	return chanGraceExit
}
