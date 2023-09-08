// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package alloc_logs

import (
	"testing"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func TestAllocLogs(t *testing.T) {

	// Wait until we have a usable cluster before running the tests.
	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 1)

	// Run our test cases.
	t.Run("TestAllocLogs_MixedFollow", testMixedFollow)
}

func testMixedFollow(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)

	// Generate our job ID which will be used for the entire test.
	jobID := "alloc-logs-mixed-follow-" + uuid.Short()
	jobIDs := []string{jobID}

	// Ensure jobs are cleaned.
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	allocStubs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "./input/mixed-output.nomad", jobID, "")
	must.Len(t, 1, allocStubs)

	// Run the alloc logs command which we expect to capture both stdout and
	// stderr logs. The command will reach its timeout and therefore return an
	// error. We want to ignore this, as it's expected. Any other error is
	// terminal.
	out, err := e2eutil.Command("nomad", "alloc", "logs", "-f", allocStubs[0].ID)
	if err != nil {
		must.ErrorContains(t, err, "failed: signal: killed")
	}
	must.StrContains(t, out, "stdout\nstderr")
}
