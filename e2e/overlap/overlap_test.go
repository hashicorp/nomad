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

	// Wait for at least 1 node to be ready and get its ID
	var node *api.Node
	testutil.Wait(t, func() (bool, error) {
		nodesList, _, err := nomadClient.Nodes().List(nil)
		if err != nil {
			return false, fmt.Errorf("error listing nodes: %v", err)
		}

		for _, n := range nodesList {
			if n.Status == "ready" {
				node, _, err = nomadClient.Nodes().Info(n.ID, nil)
				must.NoError(t, err)
				return true, nil
			}
		}

		return false, fmt.Errorf("no nodes ready before timeout; need at least 1 ready")
	})

	// Force job to fill one exact node
	getJob := func() (*api.Job, string) {
		job, err := e2eutil.Parse2(t, "testdata/overlap.nomad")
		must.NoError(t, err)
		jobID := *job.ID + uuid.Short()
		job.ID = &jobID
		job.Datacenters = []string{node.Datacenter}
		job.Constraints[1].RTarget = node.ID
		availCPU := int(node.NodeResources.Cpu.CpuShares - int64(node.ReservedResources.Cpu.CpuShares))
		job.TaskGroups[0].Tasks[0].Resources.CPU = &availCPU
		return job, *job.ID
	}
	job1, jobID1 := getJob()

	_, _, err := nomadClient.Jobs().Register(job1, nil)
	must.NoError(t, err)
	defer e2eutil.WaitForJobStopped(t, nomadClient, jobID1)

	var origAlloc *api.AllocationListStub
	testutil.Wait(t, func() (bool, error) {
		a, _, err := nomadClient.Jobs().Allocations(jobID1, false, nil)
		must.NoError(t, err)
		if n := len(a); n == 0 {
			return false, fmt.Errorf("timed out before an allocation was found for %s", jobID1)
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

	// Start replacement job and assert it is blocked
	job2, jobID2 := getJob()
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
		t.Logf("Sleeping for the rest of the shutdown_delay (%.3s/%s)",
			sleepyTime, job1.TaskGroups[0].Tasks[0].ShutdownDelay)
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
		a, _, err := nomadClient.Jobs().Allocations(jobID2, false, nil)
		must.NoError(t, err)
		if n := len(a); n == 0 {
			return false, fmt.Errorf("timed out before an allocation was found for %s", jobID2)
		}
		must.Len(t, 1, a)

		return a[0].ClientStatus == "running", fmt.Errorf("timed out before alloc %s for %s was running: %s",
			a[0].ID, jobID2, a[0].ClientStatus)
	})
}
