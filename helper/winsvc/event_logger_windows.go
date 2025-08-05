// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"fmt"
	"io"

	"golang.org/x/sys/windows/svc/eventlog"
)

// NewEventLogger creates a new event logger instance
func NewEventLogger(level string) (io.WriteCloser, error) {
	evtLog, err := eventlog.Open(WINDOWS_SERVICE_NAME)
	if err != nil {
		return nil, fmt.Errorf("Failed to open Windows eventlog: %w", err)
	}

	return &eventLogger{
		evtLog: evtLog,
		level:  EventlogLevelFromString(level),
	}, nil
}
