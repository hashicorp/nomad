// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package podman

import (
	"testing"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func TestPodman(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	t.Run("testBasic", testBasic)
}

func testBasic(t *testing.T) {
	nomad := e2eutil.NomadClient(t)
	jobID := "podman-basic-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// start job
	e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/podman_basic.hcl", jobID, "")

	// get alloc id
	allocID := e2eutil.SingleAllocID(t, jobID, "", 0)

	// check logs for redis startup
	logs, err := e2eutil.AllocTaskLogs(allocID, "redis", e2eutil.LogsStdOut)
	must.NoError(t, err)
	must.StrContains(t, logs, "oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo")
}
