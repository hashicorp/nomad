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

// NewHookError returns an implementation of a HookError with an underlying err
// and a pre-formatted task event.
// If the taskEvent is nil, then we won't attempt to generate one during error
// handling.
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
