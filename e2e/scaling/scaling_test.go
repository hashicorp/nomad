// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scaling

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

const defaultNS = "default"

func TestScaling(t *testing.T) {

	nomad := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	// Run our test cases.
	t.Run("TestScaling_Basic", testScalingBasic)
	t.Run("TestScaling_Namespaces", testScalingNamespaces)
	t.Run("TestScaling_System", testScalingSystemJob)
}

func testScalingBasic(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	jobID := "scaling-basic-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.MaybeCleanupJobsAndGC(&jobIDs))

	// start job
	allocs := e2eutil.RegisterAndWaitForAllocs(t,
		nomad, "./input/namespace_default_1.nomad.hcl", jobID, "")
	must.Len(t, 2, allocs, must.Sprint("expected 2 allocs"))

	// Ensure we wait for the deployment to finish, otherwise scaling will fail.
	must.NoError(t, e2eutil.WaitForLastDeploymentStatus(jobID, defaultNS, "successful", nil))

	// Simple scaling action.
	testMeta := map[string]any{"scaling-e2e-test": "value"}
	scaleResp, _, err := nomad.Jobs().Scale(
		jobID, "horizontally_scalable", pointer.Of(3),
		"Nomad e2e testing", false, testMeta, nil)
	must.NoError(t, err)
	must.NotEq(t, "", scaleResp.EvalID)
	must.NoError(t, e2eutil.WaitForAllocStatusExpected(jobID, defaultNS, []string{"running", "running", "running"}),
		must.Sprint("job should be running with 3 allocs"))

	// Ensure we wait for the deployment to finish, otherwise scaling will
	// fail for this reason.
	must.NoError(t, e2eutil.WaitForLastDeploymentStatus(jobID, defaultNS, "successful", nil))

	// Attempt break break the policy min/max parameters.
	_, _, err = nomad.Jobs().Scale(
		jobID, "horizontally_scalable", pointer.Of(4),
		"Nomad e2e testing", false, nil, nil)
	must.ErrorContains(t, err, "group count was greater than scaling policy maximum")
	_, _, err = nomad.Jobs().Scale(
		jobID, "horizontally_scalable", pointer.Of(1),
		"Nomad e2e testing", false, nil, nil)
	must.ErrorContains(t, err, "group count was less than scaling policy minimum")

	// Check the scaling events.
	statusResp, _, err := nomad.Jobs().ScaleStatus(jobID, nil)
	must.NoError(t, err)
	must.Len(t, 1, statusResp.TaskGroups["horizontally_scalable"].Events)
	must.Eq(t, testMeta, statusResp.TaskGroups["horizontally_scalable"].Events[0].Meta)

	// Remove the job.
	_, _, err = nomad.Jobs().Deregister(jobID, true, nil)
	must.NoError(t, err)
	must.NoError(t, nomad.System().GarbageCollect())

	// Attempt job registrations where the group count violates the policy
	// min/max parameters.
	err = e2eutil.Register(jobID, "input/namespace_default_2.nomad.hcl")
	must.ErrorContains(t, err, "task group count must not be greater than maximum count")
	must.Error(t, e2eutil.Register(jobID, "input/namespace_default_3.nomad.hcl"))
}

func testScalingNamespaces(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	// Create our non-default namespace.
	ANS := "NamespaceScalingTestA"
	_, err := e2eutil.Command("nomad", "namespace", "apply", ANS)
	must.NoError(t, err, must.Sprint("could not create namespace"))
	e2eutil.CleanupCommand(t, "nomad namespace delete %s", ANS)

	defaultJobID := "test-scaling-default-" + uuid.Generate()[0:8]
	aJobID := "test-scaling-a-" + uuid.Generate()[0:8]

	// Register and wait for the job deployments to succeed.
	must.NoError(t, e2eutil.Register(defaultJobID, "input/namespace_default_1.nomad.hcl"))
	must.NoError(t, e2eutil.Register(aJobID, "input/namespace_a_1.nomad.hcl"))
	must.NoError(t, e2eutil.WaitForLastDeploymentStatus(defaultJobID, defaultNS, "successful", nil))
	must.NoError(t, e2eutil.WaitForLastDeploymentStatus(aJobID, ANS, "successful", nil))

	t.Cleanup(e2eutil.MaybeCleanupNamespacedJobsAndGC(ANS, []string{aJobID}))
	t.Cleanup(e2eutil.MaybeCleanupJobsAndGC(&[]string{defaultJobID}))

	// Setup the WriteOptions for each namespace.
	defaultWriteOpts := api.WriteOptions{Namespace: defaultNS}
	aWriteOpts := api.WriteOptions{Namespace: ANS}

	// We shouldn't be able to trigger scaling across the namespace boundary.
	_, _, err = nomad.Jobs().Scale(
		defaultJobID, "horizontally_scalable", pointer.Of(3),
		"Nomad e2e testing", false, nil, &aWriteOpts)
	must.ErrorContains(t, err, "not found")
	_, _, err = nomad.Jobs().Scale(
		aJobID, "horizontally_scalable", pointer.Of(3),
		"Nomad e2e testing", false, nil, &defaultWriteOpts)
	must.ErrorContains(t, err, "not found")

	// We should be able to trigger scaling when using the correct namespace,
	// duh.
	_, _, err = nomad.Jobs().Scale(
		defaultJobID, "horizontally_scalable", pointer.Of(3),
		"Nomad e2e testing", false, nil, &defaultWriteOpts)
	must.NoError(t, err)
	_, _, err = nomad.Jobs().Scale(
		aJobID, "horizontally_scalable", pointer.Of(3),
		"Nomad e2e testing", false, nil, &aWriteOpts)
	must.NoError(t, err)
}

func testScalingSystemJob(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	// Register a system job with a scaling policy without a group count, it
	// should default to 1 per node.

	jobID := "test-scaling-" + uuid.Generate()[0:8]
	e2eutil.RegisterAndWaitForAllocs(t, nomad,
		"input/namespace_default_system.nomad.hcl", jobID, "")

	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &[]string{jobID}))

	jobs := nomad.Jobs()
	initialAllocs, _, err := jobs.Allocations(jobID, true, nil)
	must.NoError(t, err)

	// A system job will spawn an allocation per feasible node, we need to know
	// how many nodes there are to know how many allocations to expect.
	nodeStubList, _, err := nomad.Nodes().List(
		&api.QueryOptions{
			Namespace: "default",
			Params:    map[string]string{"os": "true"},
			Filter:    `Attributes["os.name"] == "ubuntu"`,
		})
	must.NoError(t, err)
	numberOfNodes := len(nodeStubList)

	must.Len(t, numberOfNodes, initialAllocs)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(initialAllocs)

	// Wait for allocations to get past initial pending state
	e2eutil.WaitForAllocsNotPending(t, nomad, allocIDs)

	// Try to scale beyond 1
	testMeta := map[string]any{"scaling-e2e-test": "value"}
	scaleResp, _, err := nomad.Jobs().Scale(jobID, "system_job_group", pointer.Of(3),
		"Nomad e2e testing", false, testMeta, nil)

	must.ErrorContains(t, err, "can only be scaled between 0 and 1")
	must.Nil(t, scaleResp)

	// The same allocs should be running.
	jobs = nomad.Jobs()
	allocs1, _, err := jobs.Allocations(jobID, true, nil)
	must.NoError(t, err)

	must.Eq(t, len(initialAllocs), len(allocs1))
	for i, a := range allocs1 {
		must.Eq(t, a.ID, initialAllocs[i].ID)
	}

	// Scale down to 0
	testMeta = map[string]any{"scaling-e2e-test": "value"}
	scaleResp, _, err = nomad.Jobs().Scale(jobID, "system_job_group", pointer.Of(0),
		"Nomad e2e testing", false, testMeta, nil)
	must.NoError(t, err)
	must.NotEq(t, "", scaleResp.EvalID)

	// Wait until allocs all stop
	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			allocs, _, err := jobs.Allocations(jobID, false, nil)
			must.NoError(t, err)
			stoppedAllocs := filterAllocsByDesiredStatus(
				structs.AllocDesiredStatusStop, allocs)
			return len(stoppedAllocs) == numberOfNodes
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(100*time.Millisecond),
	), must.Sprint("allocs did not stop"))

	// Scale up to 1 again
	testMeta = map[string]any{"scaling-e2e-test": "value"}
	scaleResp, _, err = nomad.Jobs().Scale(jobID, "system_job_group", pointer.Of(1),
		"Nomad e2e testing", false, testMeta, nil)
	must.NoError(t, err)
	must.NotEq(t, "", scaleResp.EvalID)

	// Wait for new allocation to get past initial pending state
	e2eutil.WaitForAllocsNotPending(t, nomad, allocIDs)

	// Assert job is still running and there is a running allocation again
	allocs, _, err := jobs.Allocations(jobID, true, nil)
	must.NoError(t, err)
	must.Len(t, numberOfNodes*2, allocs)
	must.Len(t, numberOfNodes,
		filterAllocsByDesiredStatus(structs.AllocDesiredStatusStop, allocs))
	must.Len(t, numberOfNodes,
		filterAllocsByDesiredStatus(structs.AllocDesiredStatusRun, allocs))
}

func filterAllocsByDesiredStatus(status string, allocs []*api.AllocationListStub) []*api.AllocationListStub {
	res := []*api.AllocationListStub{}

	for _, a := range allocs {
		if a.DesiredStatus == status {
			res = append(res, a)
		}
	}

	return res
}
