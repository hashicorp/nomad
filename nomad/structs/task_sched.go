// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/cronexpr"
)

// TaskScheduleState represents the scheduled execution state of a task (Enterprise)
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
		return NewTaskEvent(TaskPausing).
			SetDisplayMessage("Pausing due to override")
	case TaskScheduleStateSchedPause:
		return NewTaskEvent(TaskPausing).
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
	// TaskScheduleStateSchedResume is a transitory state that will become
	// either SchedPause or (sched) Run
	TaskScheduleStateSchedResume TaskScheduleState = "schedule_resume"
)

// TaskSchedule allows specifying a time based execution schedule for tasks.
//
// Enterprise only.
type TaskSchedule struct {
	Cron *TaskScheduleCron
}

func (t *TaskSchedule) Validate() error {
	if t.Cron == nil {
		return errors.New("must specify cron block")
	}

	const (
		startFields     = 6
		endFields       = 2
		restrictedChars = "/,"
	)

	if strings.Count(t.Cron.Start, " ") != (startFields - 1) {
		return fmt.Errorf("cron.start must contain %d fields", startFields)
	}
	if strings.Count(t.Cron.End, " ") != (endFields - 1) {
		return fmt.Errorf("cron.end must contain %d fields", endFields)
	}
	if strings.ContainsAny(t.Cron.Start, restrictedChars) {
		return fmt.Errorf("cron.start must not contain %q", restrictedChars)
	}
	if strings.ContainsAny(t.Cron.End, restrictedChars) {
		return fmt.Errorf("cron.end must not contain %q", restrictedChars)
	}

	return nil
}

func (t *TaskSchedule) Next(from time.Time) (start, end time.Duration, err error) {
	return t.Cron.Next(from)
}

type TaskScheduleCron struct {
	// Start is a stripped-down cron syntax, e.g.
	// "0 30 9 * * MON-FRI *"
	// is weekdays @ 09:30:00
	Start string
	// End is the end time in "{minute} {hour}" format, e.g.
	// "30 9"
	// is 09:30 AM
	// The End time *must* come after the Start time.
	// If you need something to happen overnight,
	// you may change the Timezone.
	End string
	// Timezone is the zone of time.
	Timezone string
}

func (t TaskScheduleCron) String() string {
	return fmt.Sprintf("<Start='%s', End='%s', Timezone='%s'>",
		t.Start, t.End, t.Timezone)
}

func (t TaskScheduleCron) GetTimezone() string {
	if t.Timezone == "" {
		return "Local" // https://pkg.go.dev/time#LoadLocation
	}
	return t.Timezone
}

func (t TaskScheduleCron) Next(from time.Time) (time.Duration, time.Duration, error) {
	// where are we?
	location, err := time.LoadLocation(t.GetTimezone())
	if err != nil {
		return 0, 0, fmt.Errorf("invalid timezone in schedule: %w", err)
	}
	from = from.In(location)

	// values should be pre-validated by this point
	start, err := cronexpr.Parse(t.Start)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start time in schedule: %q; %w", t.Start, err)
	}
	// get end time for prev to see if we're within the run schedule,
	// and from next to get the next pause time.
	end, err := cronexpr.Parse(t.End + " * * * *") // TODO: if we want to exclude seconds, prefix "* " + t.End here
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end time in schedule: %q; %w", t.End, err)
	}

	startNext := start.Next(from)
	// we'll check the previous start to see if we are currently between it
	// and the previous run's end, i.e. it should be running right now!
	startPrev := start.Next(from.Add(-24 * time.Hour))

	// generate ends from starts, so they always come after
	endNext := end.Next(startNext)
	endPrev := end.Next(startPrev)

	// next end must be on the same day as next start
	if endNext.Day() > startNext.Day() {
		return 0, 0, fmt.Errorf("end cannot be sooner than start; end=%q, start=%q", endNext, startNext)
	}

	// we're in the midst of it right now!
	if startPrev.Before(from) && endPrev.After(from) {
		return 0, endPrev.Sub(from), nil
	}

	return startNext.Sub(from), endNext.Sub(from), nil
}
