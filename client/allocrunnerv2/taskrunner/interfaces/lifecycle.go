package interfaces

import "os"

// XXX These should probably all return an error and we should have predefined
// error types for the task not currently running
type TaskLifecycle interface {
	Restart(source, reason string, failure bool)
	Signal(source, reason string, s os.Signal) error
	Kill(source, reason string, fail bool)
}
