// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package isolation

import (
	"regexp"
	"testing"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func TestChrootFS(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	t.Run("testTaskEnvChroot", testExecUsesChroot)
	t.Run("testTaskImageChroot", testImageUsesChroot)
	t.Run("testDownloadChrootExec", testDownloadChrootExec)
}

func testExecUsesChroot(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	jobID := "exec-chroot-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// start job
	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/chroot_exec.nomad", jobID, "")
	must.Len(t, 1, allocs)
	allocID := allocs[0].ID

	// wait for allocation stopped
	e2eutil.WaitForAllocsStopped(t, nomad, []string{allocID})

	// assert log contents
	logs, err := e2eutil.AllocLogs(allocID, "", e2eutil.LogsStdOut)
	must.NoError(t, err)
	must.RegexMatch(t, regexp.MustCompile(`(?m:^/alloc\b)`), logs)
	must.RegexMatch(t, regexp.MustCompile(`(?m:^/local\b)`), logs)
	must.RegexMatch(t, regexp.MustCompile(`(?m:^/secrets\b)`), logs)
	must.StrContains(t, logs, "/bin")
}

func testImageUsesChroot(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	jobID := "docker-chroot-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// start job
	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/chroot_docker.nomad", jobID, "")
	must.Len(t, 1, allocs)
	allocID := allocs[0].ID

	// wait for allocation stopped
	e2eutil.WaitForAllocsStopped(t, nomad, []string{allocID})

	// assert log contents
	logs, err := e2eutil.AllocLogs(allocID, "", e2eutil.LogsStdOut)
	must.NoError(t, err)
	must.RegexMatch(t, regexp.MustCompile(`(?m:^/alloc\b)`), logs)
	must.RegexMatch(t, regexp.MustCompile(`(?m:^/local\b)`), logs)
	must.RegexMatch(t, regexp.MustCompile(`(?m:^/secrets\b)`), logs)
	must.StrContains(t, logs, "/bin")
}

func testDownloadChrootExec(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	jobID := "dl-chroot-exec" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// start job
	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/chroot_dl_exec.nomad", jobID, "")
	must.Len(t, 1, allocs)
	allocID := allocs[0].ID

	// wait for allocation stopped
	e2eutil.WaitForAllocsStopped(t, nomad, []string{allocID})

	allocStatuses, err := e2eutil.AllocStatuses(jobID, "")
	must.NoError(t, err)
	t.Log("DEBUG", "job_id", jobID, "allocStatuses", allocStatuses)

	allocEvents, err := e2eutil.AllocTaskEventsForJob(jobID, "")
	must.NoError(t, err)
	t.Log("DEBUG", "job_id", jobID, "allocEvents", allocEvents)

	// wait for task complete (is the alloc stopped state not enough??)
	e2eutil.WaitForAllocTaskComplete(t, nomad, allocID, "run-script")

	// assert log contents
	logs, err := e2eutil.AllocTaskLogs(allocID, "run-script", e2eutil.LogsStdOut)
	must.NoError(t, err)
	must.StrContains(t, logs, "this output is from a script")
}
