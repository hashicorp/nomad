// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package artifact

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func TestArtifact(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	t.Run("testLinux", testLinux)
	t.Run("testWindows", testWindows)
	t.Run("testLimits", testLimits)
}

// artifactCheckLogContents verifies expected logs for artifact downloader tests.
//
// each task is designed to download the artifact in some way then cat the go.mod
// file, so we just need to read the logs
//
// note: git requires the use of destination (hence no default form)
func artifactCheckLogContents(t *testing.T, nomad *api.Client, group, task string, allocations []map[string]string) {
	var allocID string
	for _, alloc := range allocations {
		if alloc["Task Group"] == group {
			allocID = alloc["ID"]
			break
		}
	}
	e2eutil.WaitForAllocStopped(t, nomad, allocID)
	t.Run(task, func(t *testing.T) {
		logs, err := e2eutil.AllocTaskLogs(allocID, task, e2eutil.LogsStdOut)
		must.NoError(t, err)
		must.StrContains(t, logs, "module github.com/hashicorp/go-set")
	})
}

func testWindows(t *testing.T) {
	nomad := e2eutil.NomadClient(t)
	jobID := "artifact-windows-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// start job
	e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/artifact_windows.nomad", jobID, "")

	// get allocations
	allocations, err := e2eutil.AllocsForJob(jobID, "")
	must.NoError(t, err)
	must.Len(t, 1, allocations)

	// assert log contents for each task
	check := func(group, task string) {
		artifactCheckLogContents(t, nomad, group, task, allocations)
	}

	check("rawexec", "rawexec_file_default")
	check("rawexec", "rawexec_file_custom")
	check("rawexec", "rawexec_zip_default")
	check("rawexec", "rawexec_zip_custom")

	// todo(shoenig) needs git on windows
	// https://github.com/hashicorp/nomad/issues/15505
	// check("rawexec", "rawexec_git_custom")
}

func testLinux(t *testing.T) {
	nomad := e2eutil.NomadClient(t)
	jobID := "artifact-linux-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// start job
	e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/artifact_linux.nomad", jobID, "")

	// get allocations
	allocations, err := e2eutil.AllocsForJob(jobID, "")
	must.NoError(t, err)
	must.Len(t, 3, allocations)

	// assert log contents for each task
	check := func(group, task string) {
		artifactCheckLogContents(t, nomad, group, task, allocations)
	}

	check("rawexec", "rawexec_file_default")
	check("rawexec", "rawexec_file_custom")
	check("rawexec", "rawexec_file_alloc_dots")
	check("rawexec", "rawexec_file_alloc_env")
	check("rawexec", "rawexec_zip_default")
	check("rawexec", "rawexec_zip_custom")
	check("rawexec", "rawexec_git_custom")

	check("exec", "exec_file_default")
	check("exec", "exec_file_custom")
	check("exec", "exec_file_alloc")
	check("exec", "exec_zip_default")
	check("exec", "exec_zip_custom")
	check("exec", "exec_git_custom")

	check("docker", "docker_file_default")
	check("docker", "docker_file_custom")
	check("docker", "docker_file_alloc")
	check("docker", "docker_zip_default")
	check("docker", "docker_zip_custom")
	check("docker", "docker_git_custom")
}

func testLimits(t *testing.T) {
	// defaults are 100GB, 4096 files; we run into the files count here

	jobID := "artifact-limits-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	err := e2eutil.Register(jobID, "./input/artifact_limits.nomad")
	must.NoError(t, err)

	err = e2eutil.WaitForAllocStatusExpected(jobID, "", []string{"failed"})
	must.NoError(t, err)

	m, err := e2eutil.AllocTaskEventsForJob(jobID, "")
	must.NoError(t, err)

	found := false
SCAN:
	for _, events := range m {
		for _, event := range events {
			for label, description := range event {
				if label == "Type" && description == "Failed Artifact Download" {
					found = true
					break SCAN
				}

			}
		}
	}
	must.True(t, found)
}
