// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package interfaces

import (
	"time"
)

// ScriptExecutor is an interface that supports Exec()ing commands in the
// driver's context. Split out of DriverHandle to ease testing.
type ScriptExecutor interface {
	Exec(timeout time.Duration, cmd string, args []string) ([]byte, int, error)
}
