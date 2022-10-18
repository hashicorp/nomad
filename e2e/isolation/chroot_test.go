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
	e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/chroot_exec.nomad", jobID, "")

	// get allocation
	allocations, err := e2eutil.AllocsForJob(jobID, "")
	must.NoError(t, err)
	must.Len(t, 1, allocations)
	allocID := allocations[0]["ID"]

	// wait for allocation stopped
	e2eutil.WaitForAllocsStopped(t, nomad, []string{allocID})

	// assert log contents
	logs, err := e2eutil.AllocLogs(allocID, e2eutil.LogsStdOut)
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
	e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/chroot_docker.nomad", jobID, "")

	// get allocation
	allocations, err := e2eutil.AllocsForJob(jobID, "")
	must.NoError(t, err)
	must.Len(t, 1, allocations)
	allocID := allocations[0]["ID"]

	// wait for allocation stopped
	e2eutil.WaitForAllocsStopped(t, nomad, []string{allocID})

	// assert log contents
	logs, err := e2eutil.AllocLogs(allocID, e2eutil.LogsStdOut)
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
	e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/chroot_dl_exec.nomad", jobID, "")

	// get allocation
	allocations, err := e2eutil.AllocsForJob(jobID, "")
	must.NoError(t, err)
	must.Len(t, 1, allocations)
	allocID := allocations[0]["ID"]

	// wait for allocation stopped
	e2eutil.WaitForAllocsStopped(t, nomad, []string{allocID})

	// assert log contents
	logs, err := e2eutil.AllocTaskLogs(allocID, "run-script", e2eutil.LogsStdOut)
	must.NoError(t, err)
	must.StrContains(t, logs, "this output is from a script")
}
