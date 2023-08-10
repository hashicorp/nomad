// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"errors"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	errTaskNotRunning = "Task not running"
)

var (
	ErrTaskNotRunning = errors.New(errTaskNotRunning)
)

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
