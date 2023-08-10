// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package overlap

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

// TestOverlap asserts that the resources used by an allocation are not
// considered free until their ClientStatus is terminal.
//
// See: https://github.com/hashicorp/nomad/issues/10440
func TestOverlap(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)

	getJob := func() (*api.Job, string) {
		job, err := e2eutil.Parse2(t, "testdata/overlap.nomad")
		must.NoError(t, err)
		jobID := *job.ID + uuid.Short()
		job.ID = &jobID
		return job, *job.ID
	}
	job1, jobID1 := getJob()

	// Register initial job that should block subsequent job's placement until
	// its shutdown_delay is up.
	_, _, err := nomadClient.Jobs().Register(job1, nil)
	must.NoError(t, err)
	defer e2eutil.WaitForJobStopped(t, nomadClient, jobID1)

	var origAlloc *api.AllocationListStub
	testutil.Wait(t, func() (bool, error) {
		time.Sleep(500 * time.Millisecond)

		a, _, err := nomadClient.Jobs().Allocations(jobID1, false, nil)
		must.NoError(t, err)
		if n := len(a); n == 0 {
			evalOut := e2eutil.DumpEvals(nomadClient, jobID1)
			return false, fmt.Errorf("timed out before an allocation was found for %s. Evals:\n%s", jobID1, evalOut)
		}
		must.Len(t, 1, a)

		origAlloc = a[0]
		return origAlloc.ClientStatus == "running", fmt.Errorf("timed out before alloc %s for %s was running: %s",
			origAlloc.ID, jobID1, origAlloc.ClientStatus)
	})

	// Stop job but don't wait for ClientStatus terminal
	_, _, err = nomadClient.Jobs().Deregister(jobID1, false, nil)
	must.NoError(t, err)
	minStopTime := time.Now().Add(job1.TaskGroups[0].Tasks[0].ShutdownDelay)

	testutil.Wait(t, func() (bool, error) {
		a, _, err := nomadClient.Allocations().Info(origAlloc.ID, nil)
		must.NoError(t, err)
		ds, cs := a.DesiredStatus, a.ClientStatus
		return ds == "stop" && cs == "running", fmt.Errorf("expected alloc %s to be stop|running but found %s|%s",
			a.ID, ds, cs)
	})

	// Start replacement job on same node and assert it is blocked because the
	// static port is already in use.
	job2, jobID2 := getJob()
	job2.Constraints = append(job2.Constraints, api.NewConstraint("${node.unique.id}", "=", origAlloc.NodeID))
	job2.TaskGroups[0].Tasks[0].ShutdownDelay = 0 // no need on the followup

	resp, _, err := nomadClient.Jobs().Register(job2, nil)
	must.NoError(t, err)
	defer e2eutil.WaitForJobStopped(t, nomadClient, jobID2)

	testutil.Wait(t, func() (bool, error) {
		e, _, err := nomadClient.Evaluations().Info(resp.EvalID, nil)
		must.NoError(t, err)
		if e == nil {
			return false, fmt.Errorf("eval %s does not exist yet", resp.EvalID)
		}
		return e.BlockedEval != "", fmt.Errorf("expected a blocked eval to be created but found: %#v", *e)
	})

	// Wait for job1's ShutdownDelay for origAlloc.ClientStatus to go terminal
	sleepyTime := minStopTime.Sub(time.Now())
	if sleepyTime > 0 {
		t.Logf("Followup job %s blocked. Sleeping for the rest of %s's shutdown_delay (%.3s/%s)",
			*job2.ID, *job1.ID, sleepyTime, job1.TaskGroups[0].Tasks[0].ShutdownDelay)
		time.Sleep(sleepyTime)
	}

	testutil.Wait(t, func() (bool, error) {
		a, _, err := nomadClient.Allocations().Info(origAlloc.ID, nil)
		must.NoError(t, err)
		return a.ClientStatus == "complete", fmt.Errorf("expected original alloc %s to be complete but is %s",
			a.ID, a.ClientStatus)
	})

	// Assert replacement job unblocked and running
	testutil.Wait(t, func() (bool, error) {
		time.Sleep(500 * time.Millisecond)

		a, _, err := nomadClient.Jobs().Allocations(jobID2, true, nil)
		must.NoError(t, err)
		if n := len(a); n == 0 {
			evalOut := e2eutil.DumpEvals(nomadClient, jobID2)
			return false, fmt.Errorf("timed out before an allocation was found for %s; Evals:\n%s", jobID2, evalOut)
		}
		must.Len(t, 1, a)

		return a[0].ClientStatus == "running", fmt.Errorf("timed out before alloc %s for %s was running: %s",
			a[0].ID, jobID2, a[0].ClientStatus)
	})
}
