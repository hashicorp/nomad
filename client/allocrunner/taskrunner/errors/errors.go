// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package errors

import (
	"errors"

	"github.com/hashicorp/nomad/nomad/structs"
)

// ErrTaskNotRunning is returned when the underlying task is not currently
// running. It's defined here in the template package to avoid import cycles.
var ErrTaskNotRunning = errors.New("Task not running")

// NewHookError contains an underlying err and a pre-formatted task event.
func NewHookError(err error, taskEvent *structs.TaskEvent) error {
	return &hookError{
		err:       err,
		taskEvent: taskEvent,
	}
}

type hookError struct {
	taskEvent *structs.TaskEvent
	err       error
}

func (h *hookError) Error() string {
	return h.err.Error()
}

// Recoverable is true if the underlying error is recoverable.
func (h *hookError) IsRecoverable() bool {
	return structs.IsRecoverable(h.err)
}
