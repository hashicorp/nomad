// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// Statically assert error implements the expected interfaces
var _ structs.Recoverable = (*hookError)(nil)

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

	require.Equal(t, ev, herr.(*hookError).taskEvent)
	require.True(t, structs.IsRecoverable(herr))
	require.Equal(t, root.Error(), herr.Error())
	require.Equal(t, recov.Error(), herr.Error())
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

	require.Equal(t, ev, herr.(*hookError).taskEvent)
	require.False(t, structs.IsRecoverable(herr))
	require.Equal(t, err.Error(), herr.Error())
}
