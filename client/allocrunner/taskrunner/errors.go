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

// HookError is an error interface that can be used by a TaskRunner hook to
// emit custom task events when an error occurs.
type HookError interface {
	TaskEvent() *structs.TaskEvent
	Error() string
}

// NewHookError returns an implementation of a HookError with an underlying err
// and a pre-formatted task event.
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

func (h *hookError) TaskEvent() *structs.TaskEvent {
	return h.taskEvent
}

func (h *hookError) Error() string {
	return h.err.Error()
}
