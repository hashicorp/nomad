// +build deprecated

package taskrunner

// Name returns the name of the task
func (r *TaskRunner) Name() string {
	if r == nil || r.task == nil {
		return ""
	}

	return r.task.Name
}

// IsLeader returns whether the task is a leader task
func (r *TaskRunner) IsLeader() bool {
	if r == nil || r.task == nil {
		return false
	}

	return r.task.Leader
}
