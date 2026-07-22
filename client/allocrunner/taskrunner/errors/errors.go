// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package errors

import (
	"errors"

	"github.com/hashicorp/nomad/nomad/structs"
)

// ErrTaskNotRunning is returned when the underlying task is not currently
// running. It's defined here to avoid import cycles.
var ErrTaskNotRunning = errors.New("Task not running")

// NewHookError contains an underlying err and a pre-formatted task event.
func NewHookError(err error, taskEvent *structs.TaskEvent) error {
	return &HookError{
		Err:       err,
		TaskEvent: taskEvent,
	}
}

type HookError struct {
	TaskEvent *structs.TaskEvent
	Err       error
}

func (h *HookError) Error() string {
	return h.Err.Error()
}

// IsRecoverable is true if the underlying error is recoverable.
func (h *HookError) IsRecoverable() bool {
	return structs.IsRecoverable(h.Err)
}
