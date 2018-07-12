package taskrunner

import "os"

// XXX These should probably all return an error and we should have predefined
// error types for the task not currently running
type TaskLifecycle interface {
	Restart(source, reason string, failure bool)
	Signal(source, reason string, s os.Signal) error
	Kill(source, reason string, fail bool)
}

func (tr *TaskRunner) Restart(source, reason string, failure bool) {
	// TODO
}

func (tr *TaskRunner) Signal(source, reason string, s os.Signal) error {
	// TODO
	return nil
}

func (tr *TaskRunner) Kill(source, reason string, fail bool) {
	// TODO
}
