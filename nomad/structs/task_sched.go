// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

type TaskScheduleState string

func (t TaskScheduleState) Stop() bool {
	switch t {
	case TaskScheduleStateForcePause:
		return true
	case TaskScheduleStateSchedPause:
		return true
	}

	return false
}

func (t TaskScheduleState) Event() *TaskEvent {
	switch t {
	case TaskScheduleStateForcePause:
		return NewTaskEvent(TaskKilling).
			SetDisplayMessage("Pausing due to override")
	case TaskScheduleStateSchedPause:
		return NewTaskEvent(TaskKilling).
			SetDisplayMessage("Pausing due to schedule")
	case TaskScheduleStateForceRun:
		return NewTaskEvent(TaskRunning).
			SetDisplayMessage("Running due to override")
	case TaskScheduleStateRun:
		return NewTaskEvent(TaskRunning).
			SetDisplayMessage("Running due to schedule")
	}

	return nil
}

const (
	TaskScheduleStateRun        TaskScheduleState = ""
	TaskScheduleStateForceRun   TaskScheduleState = "force_run"
	TaskScheduleStateSchedPause TaskScheduleState = "scheduled_pause"
	TaskScheduleStateForcePause TaskScheduleState = "force_pause"
)

type TaskSchedule struct {
	Cron *TaskScheduleCron
}

type TaskScheduleCron struct {
	Start    string
	Stop     string
	Timezone string
}
