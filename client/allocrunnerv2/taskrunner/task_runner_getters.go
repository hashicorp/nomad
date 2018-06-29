package taskrunner

import "github.com/hashicorp/nomad/nomad/structs"

func (tr *TaskRunner) Task() *structs.Task {
	tr.taskLock.RLock()
	defer tr.taskLock.RUnlock()
	return tr.task
}
