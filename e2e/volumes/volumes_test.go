// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package volumes

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/testutil"
)

const ns = ""

// TestVolumeMounts exercises host volume and Docker volume functionality for
// the exec and docker task driver, particularly around mounting locations
// within the container and how this is exposed to the user.
func TestVolumeMounts(t *testing.T) {

	nomad := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	jobIDs := []string{}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	jobID := "test-node-drain-" + uuid.Short()
	require.NoError(t, e2eutil.Register(jobID, "./input/volumes.nomad"))
	jobIDs = append(jobIDs, jobID)

	expected := []string{"running"}
	require.NoError(t, e2eutil.WaitForAllocStatusExpected(jobID, ns, expected),
		"job should be running")

	allocs, err := e2eutil.AllocsForJob(jobID, ns)
	require.NoError(t, err, "could not get allocs for job")
	allocID := allocs[0]["ID"]
	nodeID := allocs[0]["Node ID"]

	cmdToExec := fmt.Sprintf("cat /tmp/foo/%s", allocID)

	out, err := e2eutil.AllocExec(allocID, "docker_task", cmdToExec, ns, nil)
	require.NoError(t, err, "could not exec into task: docker_task")
	require.Equal(t, allocID+"\n", out, "alloc data is missing from docker_task")

	out, err = e2eutil.AllocExec(allocID, "exec_task", cmdToExec, ns, nil)
	require.NoError(t, err, "could not exec into task: exec_task")
	require.Equal(t, out, allocID+"\n", "alloc data is missing from exec_task")

	err = e2eutil.StopJob(jobID)
	require.NoError(t, err, "could not stop job")

	// modify the job so that we make sure it's placed back on the same host.
	// we want to be able to verify that the data from the previous alloc is
	// still there
	job, err := jobspec.ParseFile("./input/volumes.nomad")
	require.NoError(t, err)
	job.ID = &jobID
	job.Constraints = []*api.Constraint{
		{
			LTarget: "${node.unique.id}",
			RTarget: nodeID,
			Operand: "=",
		},
	}
	_, _, err = nomad.Jobs().Register(job, nil)
	require.NoError(t, err, "could not register updated job")

	testutil.WaitForResultRetries(5000, func() (bool, error) {
		time.Sleep(time.Millisecond * 100)
		allocs, err = e2eutil.AllocsForJob(jobID, ns)
		if err != nil {
			return false, err
		}
		if len(allocs) < 2 {
			return false, fmt.Errorf("no new allocation for %v: %v", jobID, allocs)
		}

		return true, nil
	}, func(e error) {
		require.NoError(t, e, "failed to get new alloc")
	})

	newAllocID := allocs[0]["ID"]

	newCmdToExec := fmt.Sprintf("cat /tmp/foo/%s", newAllocID)

	out, err = e2eutil.AllocExec(newAllocID, "docker_task", cmdToExec, ns, nil)
	require.NoError(t, err, "could not exec into task: docker_task")
	require.Equal(t, out, allocID+"\n", "previous alloc data is missing from docker_task")

	out, err = e2eutil.AllocExec(newAllocID, "docker_task", newCmdToExec, ns, nil)
	require.NoError(t, err, "could not exec into task: docker_task")
	require.Equal(t, out, newAllocID+"\n", "new alloc data is missing from docker_task")

	out, err = e2eutil.AllocExec(newAllocID, "exec_task", cmdToExec, ns, nil)
	require.NoError(t, err, "could not exec into task: exec_task")
	require.Equal(t, out, allocID+"\n", "previous alloc data is missing from exec_task")

	out, err = e2eutil.AllocExec(newAllocID, "exec_task", newCmdToExec, ns, nil)
	require.NoError(t, err, "could not exec into task: exec_task")
	require.Equal(t, out, newAllocID+"\n", "new alloc data is missing from exec_task")
}
