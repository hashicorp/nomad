// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package errors

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// Statically assert error implements the expected interfaces
var _ structs.Recoverable = (*HookError)(nil)

// TestHookError_Recoverable asserts that a NewHookError is recoverable if
// passed a recoverable error.
func TestHookError_Recoverable(t *testing.T) {
	ci.Parallel(t)

	// Create root error
	root := errors.New("test error")

	// Make it recoverable
	recov := structs.NewRecoverableError(root, true)

	// Create a fake task event
	ev := structs.NewTaskEvent("test event")

	herr := NewHookError(recov, ev)

	must.Eq(t, ev, herr.(*HookError).TaskEvent)
	must.True(t, structs.IsRecoverable(herr))
	must.Eq(t, root.Error(), herr.Error())
	must.Eq(t, recov.Error(), herr.Error())
}

// TestHookError_Unrecoverable asserts that a NewHookError is not recoverable
// unless it is passed a recoverable error.
func TestHookError_Unrecoverable(t *testing.T) {
	ci.Parallel(t)

	// Create error
	err := errors.New("test error")

	// Create a fake task event
	ev := structs.NewTaskEvent("test event")

	herr := NewHookError(err, ev)

	must.Eq(t, ev, herr.(*HookError).TaskEvent)
	must.False(t, structs.IsRecoverable(herr))
	must.Eq(t, err.Error(), herr.Error())
}
