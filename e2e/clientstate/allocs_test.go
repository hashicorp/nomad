// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package clientstate

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestClientAllocs(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	t.Run("testAllocZombie", testAllocZombie)
}

// testAllocZombie ensures that a restart of a dead allocation does not cause
// it to come back to life in a not-quite alive state.
//
// https://github.com/hashicorp/nomad/issues/17079
func testAllocZombie(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	jobID := "alloc-zombie-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// start the job and wait for alloc to become failed
	err := e2eutil.Register(jobID, "./input/alloc_zombie.nomad")
	must.NoError(t, err)

	allocID := e2eutil.SingleAllocID(t, jobID, "", 0)

	// wait for alloc to be marked as failed
	e2eutil.WaitForAllocStatus(t, nomad, allocID, "failed")

	// wait for additional failures to know we got rescheduled
	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			statuses, err := e2eutil.AllocStatusesRescheduled(jobID, "")
			must.NoError(t, err)
			return len(statuses) > 2
		}),
		wait.Timeout(1*time.Minute),
		wait.Gap(1*time.Second),
	))

	// now attempt to restart our initial allocation
	// which should do nothing but give us an error
	output, err := e2eutil.Command("nomad", "alloc", "restart", allocID)
	must.ErrorContains(t, err, "restart of an alloc that should not run")
	must.StrContains(t, output, "Failed to restart allocation")
}
