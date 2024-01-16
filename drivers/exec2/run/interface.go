// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package run represents the implementation of the exec2 driver, used to run
// commands in a safe and secure way.
package run

import "context"

// Interface represents the actions that can be invoked upon an implementation
// of the exec2 driver.
type Interface interface {
	// Start the process.
	Start(context.Context) error

	// PID returns the process ID associated with the runner.
	//
	// Must only be called after Start.
	PID() int

	// Wait on the process (until completion).
	//
	// Must only be called after Start.
	Wait() error

	// Stats returns current resource utilization.
	//
	// Must only be called after Start.
	Stats() resources.Utilization
}
