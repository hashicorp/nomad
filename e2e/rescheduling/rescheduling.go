// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package rescheduling

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"time"

	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/testutil"
)

const ns = ""

type RescheduleE2ETest struct {
	framework.TC
	jobIds []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Rescheduling",
		CanRunLocal: true,
		Consul:      true,
		Cases: []framework.TestCase{
			new(RescheduleE2ETest),
		},
	})

}

func (tc *RescheduleE2ETest) BeforeAll(f *framework.F) {
	e2e.WaitForLeader(f.T(), tc.Nomad())
	e2e.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *RescheduleE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIds {
		err := e2e.StopJob(id, "-purge")
		f.Assert().NoError(err)
	}
	tc.jobIds = []string{}
	_, err := e2e.Command("nomad", "system", "gc")
	f.Assert().NoError(err)
}

// TestNoReschedule runs a job that should fail and never reschedule
func (tc *RescheduleE2ETest) TestNoReschedule(f *framework.F) {
	jobID := "test-no-reschedule-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "rescheduling/input/norescheduling.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"failed", "failed", "failed"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(jobID, ns, expected),
		"should have exactly 3 failed allocs",
	)
}

// TestNoRescheduleSystem runs a system job that should fail and never reschedule
func (tc *RescheduleE2ETest) TestNoRescheduleSystem(f *framework.F) {
	jobID := "test-reschedule-system-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "rescheduling/input/rescheduling_system.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatuses(jobID, ns) },
			func(got []string) bool {
				for _, status := range got {
					if status != "failed" {
						return false
					}
				}
				return true
			}, nil,
		),
		"should have only failed allocs",
	)
}

// TestDefaultReschedule runs a job that should reschedule after delay
func (tc *RescheduleE2ETest) TestDefaultReschedule(f *framework.F) {

	jobID := "test-default-reschedule-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "rescheduling/input/rescheduling_default.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"failed", "failed", "failed"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(jobID, ns, expected),
		"should have exactly 3 failed allocs",
	)

	// TODO(tgross): return early if "slow" isn't set
	// wait until first exponential delay kicks in and rescheduling is attempted
	time.Sleep(time.Second * 35)
	expected = []string{"failed", "failed", "failed", "failed", "failed", "failed"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(jobID, ns, expected),
		"should have exactly 6 failed allocs after 35s",
	)
}

// TestRescheduleMaxAttempts runs a job with a maximum reschedule attempts
func (tc *RescheduleE2ETest) TestRescheduleMaxAttempts(f *framework.F) {

	jobID := "test-reschedule-fail-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "rescheduling/input/rescheduling_fail.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"failed", "failed", "failed"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(jobID, ns, expected),
		"should have exactly 3 failed allocs",
	)

	job, err := jobspec.ParseFile("rescheduling/input/rescheduling_fail.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "sleep 15000"}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register updated job")

	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatuses(jobID, ns) },
			func(got []string) bool {
				for _, status := range got {
					if status == "running" {
						return true
					}
				}
				return false
			}, nil,
		),
		"should have at least 1 running alloc",
	)
}

// TestRescheduleSuccess runs a job that should be running after rescheduling
func (tc *RescheduleE2ETest) TestRescheduleSuccess(f *framework.F) {

	jobID := "test-reschedule-success-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "rescheduling/input/rescheduling_success.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatuses(jobID, ns) },
			func(got []string) bool {
				for _, status := range got {
					if status == "running" {
						return true
					}
				}
				return false
			}, nil,
		),
		"should have at least 1 running alloc",
	)
}

// TestRescheduleWithUpdate updates a running job to fail, and verifies that
// it gets rescheduled
func (tc *RescheduleE2ETest) TestRescheduleWithUpdate(f *framework.F) {

	jobID := "test-reschedule-update-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "rescheduling/input/rescheduling_update.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"running", "running", "running"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(jobID, ns, expected),
		"should have exactly 3 running allocs",
	)

	// reschedule to make fail
	job, err := jobspec.ParseFile("rescheduling/input/rescheduling_update.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register updated job")

	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatusesRescheduled(jobID, ns) },
			func(got []string) bool { return len(got) > 0 }, nil,
		),
		"should have rescheduled allocs until progress deadline",
	)
}

// TestRescheduleWithCanary updates a running job to fail, and verify that the
// canary gets rescheduled
func (tc *RescheduleE2ETest) TestRescheduleWithCanary(f *framework.F) {

	jobID := "test-reschedule-canary-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "rescheduling/input/rescheduling_canary.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"running", "running", "running"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(jobID, ns, expected),
		"should have exactly 3 running allocs",
	)

	f.NoError(
		e2e.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		"deployment should be successful")

	// reschedule to make fail
	job, err := jobspec.ParseFile("rescheduling/input/rescheduling_canary.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register updated job")

	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatusesRescheduled(jobID, ns) },
			func(got []string) bool { return len(got) > 0 }, nil,
		),
		"should have rescheduled allocs until progress deadline",
	)

	f.NoError(
		e2e.WaitForLastDeploymentStatus(jobID, ns, "running", nil),
		"deployment should be running")
}

// TestRescheduleWithCanaryAutoRevert updates a running job to fail, and
// verifies that the job gets reverted.
func (tc *RescheduleE2ETest) TestRescheduleWithCanaryAutoRevert(f *framework.F) {

	jobID := "test-reschedule-canary-revert-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "rescheduling/input/rescheduling_canary_autorevert.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"running", "running", "running"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(jobID, ns, expected),
		"should have exactly 3 running allocs",
	)

	f.NoError(
		e2e.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		"deployment should be successful")

	// reschedule to make fail
	job, err := jobspec.ParseFile("rescheduling/input/rescheduling_canary_autorevert.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register updated job")

	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatusesRescheduled(jobID, ns) },
			func(got []string) bool { return len(got) > 0 }, nil,
		),
		"should have new allocs after update",
	)

	// then we'll fail and revert
	expected = []string{"failed", "failed", "failed", "running", "running", "running"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(jobID, ns, expected),
		"should have exactly 3 running reverted allocs",
	)

	f.NoError(
		e2e.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		"deployment should be successful")
}

// TestRescheduleMaxParallel updates a job with a max_parallel config
func (tc *RescheduleE2ETest) TestRescheduleMaxParallel(f *framework.F) {

	jobID := "test-reschedule-maxp-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "rescheduling/input/rescheduling_maxp.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"running", "running", "running"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(jobID, ns, expected),
		"should have exactly 3 running allocs",
	)

	f.NoError(
		e2e.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		"deployment should be successful")

	// reschedule to make fail
	job, err := jobspec.ParseFile("rescheduling/input/rescheduling_maxp.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register updated job")

	expected = []string{"complete", "failed", "failed", "running", "running"}

	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatuses(jobID, ns) },
			func(got []string) bool {
				sort.Strings(got)
				return reflect.DeepEqual(got, expected)
			}, nil,
		),
		"should have failed allocs including rescheduled failed allocs",
	)

	f.NoError(
		e2e.WaitForLastDeploymentStatus(jobID, ns, "running", nil),
		"deployment should be running")
}

// TestRescheduleMaxParallelAutoRevert updates a job with a max_parallel
// config that will autorevert on failure
func (tc *RescheduleE2ETest) TestRescheduleMaxParallelAutoRevert(f *framework.F) {

	jobID := "test-reschedule-maxp-revert-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "rescheduling/input/rescheduling_maxp_autorevert.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"running", "running", "running"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(jobID, ns, expected),
		"should have exactly 3 running allocs",
	)

	f.NoError(
		e2e.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		"deployment should be successful")

	// reschedule to make fail
	job, err := jobspec.ParseFile("rescheduling/input/rescheduling_maxp_autorevert.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not e2e.Register updated job")

	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatusesRescheduled(jobID, ns) },
			func(got []string) bool { return len(got) > 0 }, nil,
		),
		"should have new allocs after update",
	)

	// wait for the revert
	expected = []string{"complete", "failed", "running", "running", "running"}
	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatuses(jobID, ns) },
			func(got []string) bool {
				sort.Strings(got)
				return reflect.DeepEqual(got, expected)
			}, nil,
		),
		"should have one successful, one failed, and 3 reverted allocs",
	)

	// at this point the allocs have been checked but we need to wait for the
	// deployment to be marked complete before we can assert that it's successful
	// and verify the count of deployments
	f.NoError(
		e2e.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		"most recent deployment should be successful")

	out, err := e2e.Command("nomad", "deployment", "status")
	f.NoError(err, "could not get deployment status")

	results, err := e2e.ParseColumns(out)
	f.NoError(err, "could not parse deployment status")
	statuses := map[string]int{}
	for _, row := range results {
		if row["Job ID"] == jobID {
			statuses[row["Status"]]++
		}
	}

	f.Equal(1, statuses["failed"],
		fmt.Sprintf("expected only 1 failed deployment, got:\n%s", out))
	f.Equal(2, statuses["successful"],
		fmt.Sprintf("expected 2 successful deployments, got:\n%s", out))
}

// TestRescheduleProgressDeadline verifies the progress deadline is reset with
// each healthy allocation, and that a rescheduled allocation does not.
func (tc *RescheduleE2ETest) TestRescheduleProgressDeadline(f *framework.F) {

	jobID := "test-reschedule-deadline-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "rescheduling/input/rescheduling_progressdeadline.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"running"}
	f.NoError(
		e2e.WaitForAllocStatusExpected(jobID, ns, expected),
		"should have a running allocation",
	)

	deploymentID, err := e2e.LastDeploymentID(jobID, ns)
	f.NoError(err, "couldn't look up deployment")

	oldDeadline, err := getProgressDeadline(deploymentID)
	f.NoError(err, "could not get progress deadline")
	time.Sleep(time.Second * 20)

	newDeadline, err := getProgressDeadline(deploymentID)
	f.NoError(err, "could not get new progress deadline")
	f.NotEqual(oldDeadline, newDeadline, "progress deadline should have been updated")

	f.NoError(e2e.WaitForLastDeploymentStatus(jobID, ns, "successful", nil),
		"deployment should be successful")
}

// TestRescheduleProgressDeadlineFail verifies the progress deadline is reset with
// each healthy allocation, and that a rescheduled allocation does not.
func (tc *RescheduleE2ETest) TestRescheduleProgressDeadlineFail(f *framework.F) {

	jobID := "test-reschedule-deadline-fail" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "rescheduling/input/rescheduling_progressdeadline_fail.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	testutil.WaitForResult(func() (bool, error) {
		_, err := e2e.LastDeploymentID(jobID, ns)
		return err == nil, err
	}, func(err error) {
		f.NoError(err, "deployment wasn't created yet")
	})

	deploymentID, err := e2e.LastDeploymentID(jobID, ns)
	f.NoError(err, "couldn't look up deployment")

	oldDeadline, err := getProgressDeadline(deploymentID)
	f.NoError(err, "could not get progress deadline")
	time.Sleep(time.Second * 20)

	f.NoError(e2e.WaitForLastDeploymentStatus(jobID, ns, "failed", nil),
		"deployment should be failed")

	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatuses(jobID, ns) },
			func(got []string) bool {
				for _, status := range got {
					if status != "failed" {
						return false
					}
				}
				return true
			}, nil,
		),
		"should have only failed allocs",
	)

	newDeadline, err := getProgressDeadline(deploymentID)
	f.NoError(err, "could not get new progress deadline")
	f.Equal(oldDeadline, newDeadline, "progress deadline should not have been updated")
}

func getProgressDeadline(deploymentID string) (time.Time, error) {

	out, err := e2e.Command("nomad", "deployment", "status", deploymentID)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not get deployment status: %v\n%v", err, out)
	}

	section, err := e2e.GetSection(out, "Deployed")
	if err != nil {
		return time.Time{}, fmt.Errorf("could not find Deployed section: %w", err)
	}

	rows, err := e2e.ParseColumns(section)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse Deployed section: %w", err)
	}

	layout := "2006-01-02T15:04:05Z07:00" // taken from command/helpers.go
	raw := rows[0]["Progress Deadline"]
	return time.Parse(layout, raw)
}
