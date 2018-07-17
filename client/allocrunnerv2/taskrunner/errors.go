package taskrunner

import "errors"

const (
	errTaskNotRunning = "Task not running"
)

var (
	ErrTaskNotRunning = errors.New(errTaskNotRunning)
)
