package rescheduling

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
)

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
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *RescheduleE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIds {
		_, err := e2eutil.Command("nomad", "job", "stop", "-purge", id)
		f.NoError(err)
	}
	tc.jobIds = []string{}
	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

// TestNoReschedule runs a job that should fail and never reschedule
func (tc *RescheduleE2ETest) TestNoReschedule(f *framework.F) {
	jobID := "test-no-reschedule-" + uuid.Generate()[0:8]
	register(f, "rescheduling/input/norescheduling.nomad", jobID)
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"failed", "failed", "failed"}
	err := waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool { return reflect.DeepEqual(got, expected) },
	)
	f.NoError(err, "should have exactly 3 failed allocs")
}

// TestNoRescheduleSystem runs a system job that should fail and never reschedule
func (tc *RescheduleE2ETest) TestNoRescheduleSystem(f *framework.F) {
	jobID := "test-reschedule-system-" + uuid.Generate()[0:8]
	register(f, "rescheduling/input/rescheduling_system.nomad", jobID)
	tc.jobIds = append(tc.jobIds, jobID)

	err := waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool {
			for _, status := range got {
				if status != "failed" {
					return false
				}
			}
			return true
		},
	)
	f.NoError(err, "should have only failed allocs")
}

// TestDefaultReschedule runs a job that should reschedule after delay
func (tc *RescheduleE2ETest) TestDefaultReschedule(f *framework.F) {

	jobID := "test-default-reschedule-" + uuid.Generate()[0:8]
	register(f, "rescheduling/input/rescheduling_default.nomad", jobID)
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"failed", "failed", "failed"}
	err := waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool { return reflect.DeepEqual(got, expected) },
	)
	f.NoError(err, "should have exactly 3 failed allocs")

	// TODO(tgross): return early if "slow" isn't set
	// wait until first exponential delay kicks in and rescheduling is attempted
	time.Sleep(time.Second * 35)
	expected = []string{"failed", "failed", "failed", "failed", "failed", "failed"}
	err = waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool { return reflect.DeepEqual(got, expected) },
	)
	f.NoError(err, "should have exactly 6 failed allocs after 35s")
}

// TestRescheduleMaxAttempts runs a job with a maximum reschedule attempts
func (tc *RescheduleE2ETest) TestRescheduleMaxAttempts(f *framework.F) {

	jobID := "test-reschedule-fail-" + uuid.Generate()[0:8]
	register(f, "rescheduling/input/rescheduling_fail.nomad", jobID)
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"failed", "failed", "failed"}
	err := waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool { return reflect.DeepEqual(got, expected) },
	)
	f.NoError(err, "should have exactly 3 failed allocs")

	job, err := jobspec.ParseFile("rescheduling/input/rescheduling_fail.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "sleep 15000"}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register updated job")

	err = waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool {
			for _, status := range got {
				if status == "running" {
					return true
				}
			}
			return false
		},
	)
	f.NoError(err, "should have at least 1 running alloc")
}

// TestRescheduleSuccess runs a job that should be running after rescheduling
func (tc *RescheduleE2ETest) TestRescheduleSuccess(f *framework.F) {

	jobID := "test-reschedule-success-" + uuid.Generate()[0:8]
	register(f, "rescheduling/input/rescheduling_success.nomad", jobID)
	tc.jobIds = append(tc.jobIds, jobID)

	err := waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool {
			for _, status := range got {
				if status == "running" {
					return true
				}
			}
			return false
		},
	)
	f.NoError(err, "should have at least 1 running alloc")
}

// TestRescheduleWithUpdate updates a running job to fail, and verifies that
// it gets rescheduled
func (tc *RescheduleE2ETest) TestRescheduleWithUpdate(f *framework.F) {

	jobID := "test-reschedule-update-" + uuid.Generate()[0:8]
	register(f, "rescheduling/input/rescheduling_update.nomad", jobID)
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"running", "running", "running"}
	err := waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool { return reflect.DeepEqual(got, expected) },
	)
	f.NoError(err, "should have exactly 3 running allocs")

	// reschedule to make fail
	job, err := jobspec.ParseFile("rescheduling/input/rescheduling_update.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register updated job")

	err = waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatusesRescheduled(f, jobID), nil },
		func(got []string) bool { return len(got) > 0 },
	)
	f.NoError(err, "should have rescheduled allocs until progress deadline")
}

// TestRescheduleWithCanary updates a running job to fail, and verify that the
// canary gets rescheduled
func (tc *RescheduleE2ETest) TestRescheduleWithCanary(f *framework.F) {

	jobID := "test-reschedule-canary-" + uuid.Generate()[0:8]
	register(f, "rescheduling/input/rescheduling_canary.nomad", jobID)
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"running", "running", "running"}
	err := waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool { return reflect.DeepEqual(got, expected) },
	)
	f.NoError(err, "should have exactly 3 running allocs")

	err = waitForLastDeploymentStatus(f, jobID, "successful")
	f.NoError(err, "deployment should be successful")

	// reschedule to make fail
	job, err := jobspec.ParseFile("rescheduling/input/rescheduling_canary.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register updated job")

	err = waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatusesRescheduled(f, jobID), nil },
		func(got []string) bool { return len(got) > 0 },
	)
	f.NoError(err, "should have rescheduled allocs until progress deadline")

	err = waitForLastDeploymentStatus(f, jobID, "running")
	f.NoError(err, "deployment should be running")
}

// TestRescheduleWithCanary updates a running job to fail, and verifies that
// the job gets reverted
func (tc *RescheduleE2ETest) TestRescheduleWithCanaryAutoRevert(f *framework.F) {

	jobID := "test-reschedule-canary-revert-" + uuid.Generate()[0:8]
	register(f, "rescheduling/input/rescheduling_canary_autorevert.nomad", jobID)
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"running", "running", "running"}
	err := waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool { return reflect.DeepEqual(got, expected) },
	)
	f.NoError(err, "should have exactly 3 running allocs")

	err = waitForLastDeploymentStatus(f, jobID, "successful")
	f.NoError(err, "deployment should be successful")

	// reschedule to make fail
	job, err := jobspec.ParseFile("rescheduling/input/rescheduling_canary_autorevert.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register updated job")

	err = waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatusesRescheduled(f, jobID), nil },
		func(got []string) bool { return len(got) == 0 },
	)
	f.NoError(err, "should have new allocs after update")

	// then we'll fail and revert
	expected = []string{"failed", "failed", "failed", "running", "running", "running"}
	err = waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool { return reflect.DeepEqual(got, expected) },
	)
	f.NoError(err, "should have exactly 3 running reverted allocs")

	err = waitForLastDeploymentStatus(f, jobID, "successful")
	f.NoError(err, "deployment should be successful")
}

// TestRescheduleMaxParallel updates a job with a max_parallel config
func (tc *RescheduleE2ETest) TestRescheduleMaxParallel(f *framework.F) {

	jobID := "test-reschedule-maxp-" + uuid.Generate()[0:8]
	register(f, "rescheduling/input/rescheduling_maxp.nomad", jobID)
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"running", "running", "running"}
	err := waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool { return reflect.DeepEqual(got, expected) },
	)
	f.NoError(err, "should have exactly 3 running allocs")

	err = waitForLastDeploymentStatus(f, jobID, "successful")
	f.NoError(err, "deployment should be successful")

	// reschedule to make fail
	job, err := jobspec.ParseFile("rescheduling/input/rescheduling_maxp.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register updated job")

	expected = []string{"complete", "failed", "failed", "running", "running"}
	err = waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool {
			sort.Strings(got)
			return reflect.DeepEqual(got, expected)
		},
	)
	f.NoError(err, "should have failed allocs including rescheduled failed allocs")

	err = waitForLastDeploymentStatus(f, jobID, "running")
	f.NoError(err, "deployment should be running")
}

// TestRescheduleMaxParallelAutoRevert updates a job with a max_parallel
// config that will autorevert on failure
func (tc *RescheduleE2ETest) TestRescheduleMaxParallelAutoRevert(f *framework.F) {

	jobID := "test-reschedule-maxp-revert-" + uuid.Generate()[0:8]
	register(f, "rescheduling/input/rescheduling_maxp_autorevert.nomad", jobID)
	tc.jobIds = append(tc.jobIds, jobID)

	expected := []string{"running", "running", "running"}
	err := waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool { return reflect.DeepEqual(got, expected) },
	)
	f.NoError(err, "should have exactly 3 running allocs")

	err = waitForLastDeploymentStatus(f, jobID, "successful")
	f.NoError(err, "deployment should be successful")

	// reschedule to make fail
	job, err := jobspec.ParseFile("rescheduling/input/rescheduling_maxp_autorevert.nomad")
	f.NoError(err)
	job.ID = &jobID
	job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register updated job")

	err = waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatusesRescheduled(f, jobID), nil },
		func(got []string) bool { return len(got) == 0 },
	)
	f.NoError(err, "should have new allocs after update")

	// wait for the revert
	expected = []string{"complete", "failed", "running", "running", "running"}
	err = waitForAllocStatusComparison(
		func() ([]string, error) { return allocStatuses(f, jobID), nil },
		func(got []string) bool {
			sort.Strings(got)
			return reflect.DeepEqual(got, expected)
		},
	)
	f.NoError(err, "should have one successful, one failed, and 3 reverted allocs")

	out, err := e2eutil.Command("nomad", "deployment", "status")
	f.NoError(err, "could not get deployment status")

	results, err := e2eutil.ParseColumns(out)
	f.NoError(err, "could not parse deployment status")
	statuses := []string{}
	for _, row := range results {
		if row["Job ID"] == jobID {
			statuses = append(statuses, row["Status"])
		}
	}
	f.True(reflect.DeepEqual([]string{"running", "failed", "successful"}, statuses),
		fmt.Sprintf("deployment status was: %#v", statuses),
	)
}

// TestRescheduleProgressDeadline verifies a deployment succeeds by the
// progress deadline
func (tc *RescheduleE2ETest) TestRescheduleProgressDeadline(f *framework.F) {

	jobID := "test-reschedule-deadline-" + uuid.Generate()[0:8]
	register(f, "rescheduling/input/rescheduling_progressdeadline.nomad", jobID)
	tc.jobIds = append(tc.jobIds, jobID)

	// TODO(tgross): return early if "slow" isn't set
	// wait until first exponential delay kicks in and rescheduling is attempted
	time.Sleep(time.Second * 30)
	err := waitForLastDeploymentStatus(f, jobID, "successful")
	f.NoError(err, "deployment should be successful")
}
