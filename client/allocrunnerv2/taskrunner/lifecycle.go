package taskrunner

import "os"

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
