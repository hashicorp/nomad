// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package task_schedule

import (
	"fmt"
	"testing"
	"time"

	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

const jobspec = "./input/schedule.nomad.hcl"

// TestTaskSchedule tests the task{ schedule{} } block:
// https://developer.hashicorp.com/nomad/docs/job-specification/schedule
func TestTaskSchedule(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Enterprise(),
		cluster3.Leader(),
		cluster3.LinuxClients(1),
	)

	nomadClient, err := nomadapi.NewClient(nomadapi.DefaultConfig())
	must.NoError(t, err)

	t.Run("in schedule", testInSchedule)
	t.Run("in future", testInFuture)
	t.Run("job update", testJobUpdate)
	t.Run("force run", testForceRun(nomadClient))
	t.Run("force stop", testForceStop(nomadClient))
	t.Run("repeat pause", testRepeatPause(nomadClient))
	t.Run("task dies", testTaskDies(nomadClient))
}

// testInSchedule ensures a task starts when allocated in schedule,
// then is killed at the end of the schedule.
func testInSchedule(t *testing.T) {
	now := time.Now()

	// start one minute ago, end one minute from now.
	job := runJob(t, now.Add(-time.Minute), now.Add(time.Minute))

	// task should start nearly right away
	expectAllocStatus(t, job, "running", 5*time.Second, "task should start")

	// in about a minute, the task should get killed and restart
	expectAllocStatus(t, job, "pending", time.Minute+(5*time.Second), "task should be killed")

	// all in all, this is what should have happened
	expectTaskEvents(t, job, []string{
		"Received",
		"Task Setup",
		"Started",
		"Pausing",
		"Terminated",
		"Restarting",
	})
}

// testInFuture ensures a task "pauses" until the schedule starts,
// then is killed at the end.
func testInFuture(t *testing.T) {
	now := time.Now()

	// run 2 min in the future, so we can ensure it stays pending for ~a minute
	job := runJob(t, now.Add(2*time.Minute), now.Add(3*time.Minute))

	// should not start right away
	time.Sleep(5 * time.Second)
	expectAllocStatus(t, job, "pending", 0, "task should stay pending")

	logStamp(t, "wait a minute")
	time.Sleep(time.Minute)
	expectAllocStatus(t, job, "running", time.Minute+(5*time.Second), "task should start")

	expectAllocStatus(t, job, "pending", time.Minute+(5*time.Second), "task should be killed")

	expectTaskEvents(t, job, []string{
		"Received",
		"Task Setup",
		"Pausing",
		"Running",
		"Started",
		"Pausing",
		"Terminated",
		"Restarting",
	})
}

// testJobUpdate ensures job updates that change the schedule appropriately
// start or stop the task.
func testJobUpdate(t *testing.T) {
	now := time.Now()

	// schedule in future; task should not run.
	job := runJob(t, now.Add(time.Hour), now.Add(2*time.Hour))
	time.Sleep(5 * time.Second)
	expectAllocStatus(t, job, "pending", 0, "task should stay pending")

	// update the same job with a schedule that should run now;
	// task should run.
	rerunJob(t, job, now.Add(-time.Hour), now.Add(time.Hour))
	expectAllocStatus(t, job, "running", time.Minute+(5*time.Second), "task should start")

	// update the job again, putting it out of schedule;
	// task should stop.
	rerunJob(t, job, now.Add(time.Hour), now.Add(2*time.Hour))
	expectAllocStatus(t, job, "pending", time.Minute+(5*time.Second), "task should be killed")

	expectTaskEvents(t, job, []string{
		"Received",
		"Task Setup",
		"Pausing",
		"Running",
		"Started",
		"Pausing",
		"Terminated",
		"Restarting",
	})
}

// testForceRun ensures the "pause" API can force the task to run,
// even when out of schedule, then resuming the schedule should stop it again.
func testForceRun(api *nomadapi.Client) func(t *testing.T) {
	return func(t *testing.T) {
		now := time.Now()

		// schedule in future; task should not run.
		job := runJob(t, now.Add(time.Hour), now.Add(2*time.Hour))
		expectAllocStatus(t, job, "pending", 5*time.Second, "task should be placed")

		alloc := &nomadapi.Allocation{
			ID: job.AllocID("group"),
		}
		expectScheduleState(t, api, alloc, "scheduled_pause")

		// force the task to run.
		must.NoError(t,
			api.Allocations().SetPauseState(alloc, nil, "app", "run"))
		expectScheduleState(t, api, alloc, "force_run")
		expectAllocStatus(t, job, "running", 5*time.Second, "task should start")

		// resume schedule; should stop the task.
		must.NoError(t,
			api.Allocations().SetPauseState(alloc, nil, "app", "scheduled"))
		expectScheduleState(t, api, alloc, "scheduled_pause")
		expectAllocStatus(t, job, "pending", 5*time.Second, "task should stop")

		expectTaskEvents(t, job, []string{
			"Received",
			"Task Setup",
			"Pausing",
			"Running",
			"Started",
			"Pausing",
			"Terminated",
			"Restarting",
		})
	}
}

// testForceStop ensures the "pause" API can force the task to stop ("pause"),
// even when in schedule, then resuming the schedule should start the task.
func testForceStop(api *nomadapi.Client) func(t *testing.T) {
	return func(t *testing.T) {
		now := time.Now()

		// in schedule; task should run.
		job := runJob(t, now.Add(-time.Hour), now.Add(time.Hour))
		expectAllocStatus(t, job, "running", 5*time.Second, "task should start")

		alloc := &nomadapi.Allocation{
			ID: job.AllocID("group"),
		}
		expectScheduleState(t, api, alloc, "") // "" = run (scheduled)

		// force the task to stop.
		must.NoError(t,
			api.Allocations().SetPauseState(alloc, nil, "app", "pause"))
		expectScheduleState(t, api, alloc, "force_pause")
		expectAllocStatus(t, job, "pending", 5*time.Second, "task should stop")

		// resume schedule; task should resume.
		must.NoError(t,
			api.Allocations().SetPauseState(alloc, nil, "app", "scheduled"))
		expectScheduleState(t, api, alloc, "")
		expectAllocStatus(t, job, "running", 15*time.Second, "task should start")

		expectTaskEvents(t, job, []string{
			"Received",
			"Task Setup",
			"Started",
			"Pausing",
			"Terminated",
			"Restarting",
			"Running",
			"Started",
		})
	}
}

// testRepeatPause ensures that pausing a task resets the restart counter,
// so only application exits count against the restart attempts limit.
func testRepeatPause(api *nomadapi.Client) func(t *testing.T) {
	return func(t *testing.T) {
		now := time.Now()

		// schedule in future; task should not run.
		job := runJob(t, now.Add(time.Hour), now.Add(2*time.Hour))
		expectAllocStatus(t, job, "pending", 5*time.Second, "task should be placed")

		alloc := &nomadapi.Allocation{
			ID: job.AllocID("group"),
		}
		expectScheduleState(t, api, alloc, "scheduled_pause")

		// the test job only allows for 1 restart attempt, so 3 stops would
		// cause a failure if we fail to reset the restart counter (a bug)
		for x := range 3 {
			t.Run(fmt.Sprintf("attempt %d", x+1), func(t *testing.T) {
				// force the task to run.
				must.NoError(t, api.Allocations().SetPauseState(alloc, nil, "app", "run"))
				expectScheduleState(t, api, alloc, "force_run")
				expectAllocStatus(t, job, "running", 5*time.Second, "task should start")

				// force the task to stop.
				must.NoError(t, api.Allocations().SetPauseState(alloc, nil, "app", "pause"))
				expectScheduleState(t, api, alloc, "force_pause")
				expectAllocStatus(t, job, "pending", 5*time.Second, "task should stop")
			})
		}

		// this skips "Received" and "Task Setup" and an initial pause
		// because only 10 task events get stored at a time.
		expectTaskEvents(t, job, []string{
			"Running", "Started", "Pausing", "Terminated", "Restarting",
			"Running", "Started", "Pausing", "Terminated", "Restarting",
		})
	}
}

// testTaskDies tests that a task dying on its own counts against the restart
// counter (unlike repeat intentional pauses as in testRepeatPause)
func testTaskDies(api *nomadapi.Client) func(t *testing.T) {
	return func(t *testing.T) {
		now := time.Now()
		// schedule now; task should run.
		job := runJob(t, now.Add(-time.Hour), now.Add(time.Hour))
		expectAllocStatus(t, job, "running", 5*time.Second, "task should start")

		alloc := &nomadapi.Allocation{
			ID: job.AllocID("group"),
		}

		// the job has 0 restart attempts, so the first failure should be fatal.
		must.NoError(t, api.Allocations().Signal(alloc, nil, "app", "SIGTERM"))
		expectAllocStatus(t, job, "failed", 5*time.Second, "task should fail")

		expectTaskEvents(t, job, []string{
			"Received", "Task Setup",
			"Started", "Signaling", "Terminated", "Not Restarting",
		})
	}
}

/** helpers **/

// logStamp logs with a timestamp; the feature being tested is all about time.
func logStamp(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf(time.Now().UTC().Format(time.RFC3339)+" "+format, args...)
}

// runJob runs a job.
func runJob(t *testing.T, start, end time.Time) *jobs3.Submission {
	t.Helper()
	opts := jobOpts(t, start, end)
	job, _ := jobs3.Submit(t, jobspec, opts...)
	logStamp(t, "ran job %q", job.JobID())
	return job
}

// rerunJob re-runs the job with new start/end times.
func rerunJob(t *testing.T, job *jobs3.Submission, start, end time.Time) {
	t.Helper()
	opts := jobOpts(t, start, end)
	job.Rerun(opts...)
	logStamp(t, "re-ran job %q", job.JobID())
}

// jobOpts provides the options we need to (re)run the job.
func jobOpts(t *testing.T, start, end time.Time) []jobs3.Option {
	t.Helper()
	startS := start.UTC().Format("4 15 * * * *")
	endS := end.UTC().Format("4 15")
	logStamp(t, "job options: start=%q end=%q", startS, endS)
	return []jobs3.Option{
		jobs3.Var("start", startS),
		jobs3.Var("end", endS),
		jobs3.Detach(), // disable deployment checking
	}
}

// expectAllocStatus asserts that a job's alloc reaches the expected status
// before the timeout.
func expectAllocStatus(t *testing.T, job *jobs3.Submission, expect string, timeout time.Duration, message string) {
	t.Helper()

	check := func() error {
		allocs := job.Allocs()
		if len(allocs) < 1 {
			return fmt.Errorf("no allocs for job %q", job.JobID())
		}
		actual := allocs[0].ClientStatus
		if expect != actual {
			return fmt.Errorf("expect alloc status %q; got %q", expect, actual)
		}
		return nil
	}

	if timeout == 0 {
		must.NoError(t, check(), must.Sprint(message))
		return
	}

	logStamp(t, "waiting up to %s: %s", timeout, message)
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(check),
		wait.Timeout(timeout),
		wait.Gap(time.Second),
	), must.Sprintf("ran out of time waiting: %q", message))
}

// expectTaskEvents asserts a job's task events Types.
func expectTaskEvents(t *testing.T, job *jobs3.Submission, expect []string) {
	t.Helper()

	allocID := job.AllocID("group")
	events, ok := job.AllocEvents()[allocID]
	must.True(t, ok, must.Sprintf("did not find alloc in events"))

	actual := make([]string, len(events.Events))
	for i, e := range events.Events {
		actual[i] = e.Type
	}
	must.Eq(t, expect, actual)
}

// expectScheduleState asserts that the "pause" state of the allocation/task.
func expectScheduleState(t *testing.T, api *nomadapi.Client, alloc *nomadapi.Allocation, expect string) {
	t.Helper()
	actual, _, err := api.Allocations().GetPauseState(alloc, nil, "app")
	must.NoError(t, err)
	must.Eq(t, expect, actual)
}
