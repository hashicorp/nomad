// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package rescheduling

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

const ns = "default"

// Note: most of the StopJob calls in this test suite will return an
// error because the job has previously failed and we're not waiting for
// the deployment to end

// TestRescheduling_Service_NoReschedule runs a service job that should fail and never
// reschedule
func TestRescheduling_Service_NoReschedule(t *testing.T) {
	jobID := "test-no-reschedule-" + uuid.Generate()[0:8]
	must.NoError(t, e2eutil.Register(jobID, "./input/norescheduling_service.nomad"))

	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach")
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	expected := []string{"failed", "failed", "failed"}
	must.NoError(t,
		e2eutil.WaitForAllocStatusExpected(jobID, ns, expected),
		must.Sprint("should have exactly 3 failed allocs"),
	)
}

// TestRescheduling_System_NoReschedule runs a system job that should fail and never
// reschedule
func TestRescheduling_System_NoReschedule(t *testing.T) {
	jobID := "test-no-reschedule-" + uuid.Generate()[0:8]
	must.NoError(t, e2eutil.Register(jobID, "./input/norescheduling_system.nomad"))

	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach")
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	must.NoError(t,
		e2eutil.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2eutil.AllocStatuses(jobID, ns) },
			func(got []string) bool {
				for _, status := range got {
					if status != "failed" {
						return false
					}
				}
				return true
			}, nil,
		),
		must.Sprint("should have only failed allocs"),
	)
}

// TestRescheduling_Default runs a job that should reschedule after delay
func TestRescheduling_Default(t *testing.T) {
	jobID := "test-default-reschedule-" + uuid.Generate()[0:8]
	must.NoError(t, e2eutil.Register(jobID, "./input/rescheduling_default.nomad"))

	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach")
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	expected := []string{"failed", "failed", "failed"}
	must.NoError(t,
		e2eutil.WaitForAllocStatusExpected(jobID, ns, expected),
		must.Sprint("should have exactly 3 failed allocs"),
	)

	// wait until first exponential delay kicks in and rescheduling is attempted
	time.Sleep(time.Second * 35)
	expected = []string{"failed", "failed", "failed", "failed", "failed", "failed"}
	must.NoError(t,
		e2eutil.WaitForAllocStatusExpected(jobID, ns, expected),
		must.Sprint("should have exactly 6 failed allocs after 35s"),
	)
}

// TestRescheduling_MaxAttempts runs a job with a maximum reschedule attempts
func TestRescheduling_MaxAttempts(t *testing.T) {

	jobID := "test-reschedule-fail-" + uuid.Generate()[0:8]
	must.NoError(t, e2eutil.Register(jobID, "./input/rescheduling_fail.nomad"))

	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach")
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	expected := []string{"failed", "failed", "failed"}
	must.NoError(t,
		e2eutil.WaitForAllocStatusExpected(jobID, ns, expected),
		must.Sprint("should have exactly 3 failed allocs"),
	)

	job, err := jobspec.ParseFile("./input/rescheduling_fail.nomad")
	must.NoError(t, err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "sleep 15000"}

	nc := e2eutil.NomadClient(t)
	_, _, err = nc.Jobs().Register(job, nil)
	must.NoError(t, err, must.Sprint("could not register updated job"))

	must.NoError(t,
		e2eutil.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2eutil.AllocStatuses(jobID, ns) },
			func(got []string) bool {
				for _, status := range got {
					if status == "running" {
						return true
					}
				}
				return false
			}, nil,
		),
		must.Sprint("should have at least 1 running alloc"),
	)
}

// TestRescheduling_Success runs a job that should be running after rescheduling
func TestRescheduling_Success(t *testing.T) {

	jobID := "test-reschedule-success-" + uuid.Generate()[0:8]
	must.NoError(t, e2eutil.Register(jobID, "./input/rescheduling_success.nomad"))

	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach")
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	must.NoError(t,
		e2eutil.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2eutil.AllocStatuses(jobID, ns) },
			func(got []string) bool {
				for _, status := range got {
					if status == "running" {
						return true
					}
				}
				return false
			}, nil,
		),
		must.Sprint("should have at least 1 running alloc"),
	)
}

// TestRescheduling_WithUpdate updates a running job to fail, and verifies that
// it gets rescheduled
func TestRescheduling_WithUpdate(t *testing.T) {

	jobID := "test-reschedule-update-" + uuid.Generate()[0:8]
	must.NoError(t, e2eutil.Register(jobID, "./input/rescheduling_update.nomad"))

	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach")
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	expected := []string{"running", "running", "running"}
	must.NoError(t,
		e2eutil.WaitForAllocStatusExpected(jobID, ns, expected),
		must.Sprint("should have exactly 3 running allocs"),
	)

	// reschedule to make fail
	job, err := jobspec.ParseFile("./input/rescheduling_update.nomad")
	must.NoError(t, err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}

	nc := e2eutil.NomadClient(t)
	_, _, err = nc.Jobs().Register(job, nil)
	must.NoError(t, err, must.Sprint("could not register updated job"))

	must.NoError(t,
		e2eutil.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2eutil.AllocStatusesRescheduled(jobID, ns) },
			func(got []string) bool { return len(got) > 0 }, nil,
		),
		must.Sprint("should have rescheduled allocs until progress deadline"),
	)
}

// TestRescheduling_WithCanary updates a running job to fail, and verify that the
// canary gets rescheduled
func TestRescheduling_WithCanary(t *testing.T) {

	jobID := "test-reschedule-canary-" + uuid.Generate()[0:8]
	must.NoError(t, e2eutil.Register(jobID, "./input/rescheduling_canary.nomad"))

	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach")
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	expected := []string{"running", "running", "running"}
	must.NoError(t,
		e2eutil.WaitForAllocStatusExpected(jobID, ns, expected),
		must.Sprint("should have exactly 3 running allocs"),
	)

	must.NoError(t,
		e2eutil.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		must.Sprint("deployment should be successful"))

	// reschedule to make fail
	job, err := jobspec.ParseFile("./input/rescheduling_canary.nomad")
	must.NoError(t, err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}

	nc := e2eutil.NomadClient(t)
	_, _, err = nc.Jobs().Register(job, nil)
	must.NoError(t, err, must.Sprint("could not register updated job"))

	must.NoError(t,
		e2eutil.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2eutil.AllocStatusesRescheduled(jobID, ns) },
			func(got []string) bool { return len(got) > 0 }, nil,
		),
		must.Sprint("should have rescheduled allocs until progress deadline"),
	)

	must.NoError(t,
		e2eutil.WaitForLastDeploymentStatus(jobID, ns, "running", nil),
		must.Sprint("deployment should be running"))
}

// TestRescheduling_WithCanaryAutoRevert updates a running job to fail, and
// verifies that the job gets reverted.
func TestRescheduling_WithCanaryAutoRevert(t *testing.T) {

	jobID := "test-reschedule-canary-revert-" + uuid.Generate()[0:8]
	must.NoError(t, e2eutil.Register(jobID, "./input/rescheduling_canary_autorevert.nomad"))

	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach")
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	expected := []string{"running", "running", "running"}
	must.NoError(t,
		e2eutil.WaitForAllocStatusExpected(jobID, ns, expected),
		must.Sprint("should have exactly 3 running allocs"),
	)

	must.NoError(t,
		e2eutil.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		must.Sprint("deployment should be successful"))

	// reschedule to make fail
	job, err := jobspec.ParseFile("./input/rescheduling_canary_autorevert.nomad")
	must.NoError(t, err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}

	nc := e2eutil.NomadClient(t)
	_, _, err = nc.Jobs().Register(job, nil)
	must.NoError(t, err, must.Sprint("could not register updated job"))

	must.NoError(t,
		e2eutil.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2eutil.AllocStatusesRescheduled(jobID, ns) },
			func(got []string) bool { return len(got) > 0 }, nil,
		),
		must.Sprint("should have new allocs after update"),
	)

	// then we'll fail and revert
	expected = []string{"failed", "failed", "failed", "running", "running", "running"}
	must.NoError(t,
		e2eutil.WaitForAllocStatusExpected(jobID, ns, expected),
		must.Sprint("should have exactly 3 running reverted allocs"),
	)

	must.NoError(t,
		e2eutil.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		must.Sprint("deployment should be successful"))
}

// TestRescheduling_MaxParallel updates a job with a max_parallel config
func TestRescheduling_MaxParallel(t *testing.T) {

	jobID := "test-reschedule-maxp-" + uuid.Generate()[0:8]
	must.NoError(t, e2eutil.Register(jobID, "./input/rescheduling_maxp.nomad"))

	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach")
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	expected := []string{"running", "running", "running"}
	must.NoError(t,
		e2eutil.WaitForAllocStatusExpected(jobID, ns, expected),
		must.Sprint("should have exactly 3 running allocs"),
	)

	must.NoError(t,
		e2eutil.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		must.Sprint("deployment should be successful"))

	// reschedule to make fail
	job, err := jobspec.ParseFile("./input/rescheduling_maxp.nomad")
	must.NoError(t, err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}

	nc := e2eutil.NomadClient(t)
	_, _, err = nc.Jobs().Register(job, nil)
	must.NoError(t, err, must.Sprint("could not register updated job"))

	expected = []string{"complete", "failed", "failed", "running", "running"}

	must.NoError(t,
		e2eutil.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2eutil.AllocStatuses(jobID, ns) },
			func(got []string) bool {
				sort.Strings(got)
				return reflect.DeepEqual(got, expected)
			}, nil,
		),
		must.Sprint("should have failed allocs including rescheduled failed allocs"),
	)

	must.NoError(t,
		e2eutil.WaitForLastDeploymentStatus(jobID, ns, "running", nil),
		must.Sprint("deployment should be running"))
}

// TestRescheduling_MaxParallelAutoRevert updates a job with a max_parallel
// config that will autorevert on failure
func TestRescheduling_MaxParallelAutoRevert(t *testing.T) {

	jobID := "test-reschedule-maxp-revert-" + uuid.Generate()[0:8]
	must.NoError(t, e2eutil.Register(jobID, "./input/rescheduling_maxp_autorevert.nomad"))

	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach")
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	expected := []string{"running", "running", "running"}
	must.NoError(t,
		e2eutil.WaitForAllocStatusExpected(jobID, ns, expected),
		must.Sprint("should have exactly 3 running allocs"),
	)

	must.NoError(t,
		e2eutil.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		must.Sprint("deployment should be successful"))

	// reschedule to make fail
	job, err := jobspec.ParseFile("./input/rescheduling_maxp_autorevert.nomad")
	must.NoError(t, err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}

	nc := e2eutil.NomadClient(t)
	_, _, err = nc.Jobs().Register(job, nil)
	must.NoError(t, err, must.Sprint("could not e2eutil.Register updated job"))

	must.NoError(t,
		e2eutil.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2eutil.AllocStatusesRescheduled(jobID, ns) },
			func(got []string) bool { return len(got) > 0 }, nil,
		),
		must.Sprint("should have new allocs after update"),
	)

	// wait for the revert
	expected = []string{"complete", "failed", "running", "running", "running"}
	must.NoError(t,
		e2eutil.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2eutil.AllocStatuses(jobID, ns) },
			func(got []string) bool {
				sort.Strings(got)
				return reflect.DeepEqual(got, expected)
			}, nil,
		),
		must.Sprint("should have one successful, one failed, and 3 reverted allocs"),
	)

	// at this point the allocs have been checked but we need to wait for the
	// deployment to be marked complete before we can assert that it's successful
	// and verify the count of deployments
	must.NoError(t,
		e2eutil.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		must.Sprint("most recent deployment should be successful"))

	out, err := e2eutil.Command("nomad", "deployment", "status")
	must.NoError(t, err, must.Sprint("could not get deployment status"))

	results, err := e2eutil.ParseColumns(out)
	must.NoError(t, err, must.Sprint("could not parse deployment status"))
	statuses := map[string]int{}
	for _, row := range results {
		if row["Job ID"] == jobID {
			statuses[row["Status"]]++
		}
	}

	must.Eq(t, 1, statuses["failed"],
		must.Sprintf("expected only 1 failed deployment, got:\n%s", out))
	must.Eq(t, 2, statuses["successful"],
		must.Sprintf("expected 2 successful deployments, got:\n%s", out))
}

// TestRescheduling_ProgressDeadline verifies the progress deadline is reset with
// each healthy allocation, and that a rescheduled allocation does not.
func TestRescheduling_ProgressDeadline(t *testing.T) {

	jobID := "test-reschedule-deadline-" + uuid.Generate()[0:8]
	must.NoError(t, e2eutil.Register(jobID, "./input/rescheduling_progressdeadline.nomad"))

	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach")
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	expected := []string{"running"}
	must.NoError(t,
		e2eutil.WaitForAllocStatusExpected(jobID, ns, expected),
		must.Sprint("should have a running allocation"),
	)

	deploymentID, err := e2eutil.LastDeploymentID(jobID, ns)
	must.NoError(t, err, must.Sprint("couldn't look up deployment"))

	oldDeadline, err := getProgressDeadline(deploymentID)
	must.NoError(t, err, must.Sprint("could not get progress deadline"))
	time.Sleep(time.Second * 20)

	newDeadline, err := getProgressDeadline(deploymentID)
	must.NoError(t, err, must.Sprint("could not get new progress deadline"))
	must.NotEq(t, oldDeadline, newDeadline,
		must.Sprint("progress deadline should have been updated"))

	must.NoError(t, e2eutil.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		must.Sprint("deployment should be successful"))
}

// TestRescheduling_ProgressDeadlineFail verifies the progress deadline is reset with
// each healthy allocation, and that a rescheduled allocation does not.
func TestRescheduling_ProgressDeadlineFail(t *testing.T) {

	jobID := "test-reschedule-deadline-fail" + uuid.Generate()[0:8]
	must.NoError(t, e2eutil.Register(jobID, "./input/rescheduling_progressdeadline_fail.nomad"))

	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach")
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	testutil.WaitForResult(func() (bool, error) {
		_, err := e2eutil.LastDeploymentID(jobID, ns)
		return err == nil, err
	}, func(err error) {
		must.NoError(t, err, must.Sprint("deployment wasn't created yet"))
	})

	deploymentID, err := e2eutil.LastDeploymentID(jobID, ns)
	must.NoError(t, err, must.Sprint("couldn't look up deployment"))

	oldDeadline, err := getProgressDeadline(deploymentID)
	must.NoError(t, err, must.Sprint("could not get progress deadline"))
	time.Sleep(time.Second * 20)

	must.NoError(t, e2eutil.WaitForLastDeploymentStatus(jobID, ns, "failed", nil),
		must.Sprint("deployment should be failed"))

	must.NoError(t,
		e2eutil.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2eutil.AllocStatuses(jobID, ns) },
			func(got []string) bool {
				for _, status := range got {
					if status != "failed" {
						return false
					}
				}
				return true
			}, nil,
		),
		must.Sprint("should have only failed allocs"),
	)

	newDeadline, err := getProgressDeadline(deploymentID)
	must.NoError(t, err, must.Sprint("could not get new progress deadline"))
	must.Eq(t, oldDeadline, newDeadline,
		must.Sprint("progress deadline should not have been updated"))
}

func getProgressDeadline(deploymentID string) (time.Time, error) {

	out, err := e2eutil.Command("nomad", "deployment", "status", deploymentID)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not get deployment status: %v\n%v", err, out)
	}

	section, err := e2eutil.GetSection(out, "Deployed")
	if err != nil {
		return time.Time{}, fmt.Errorf("could not find Deployed section: %w", err)
	}

	rows, err := e2eutil.ParseColumns(section)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse Deployed section: %w", err)
	}

	layout := "2006-01-02T15:04:05Z07:00" // taken from command/helpers.go
	raw := rows[0]["Progress Deadline"]
	return time.Parse(layout, raw)
}
