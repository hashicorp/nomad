// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scaling

import (
	"os"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

type ScalingE2ETest struct {
	framework.TC
	namespaceIDs     []string
	namespacedJobIDs [][2]string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Scaling",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(ScalingE2ETest),
		},
	})
}

func (tc *ScalingE2ETest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *ScalingE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, namespacedJob := range tc.namespacedJobIDs {
		err := e2eutil.StopJob(namespacedJob[1], "-purge", "-namespace",
			namespacedJob[0])
		f.NoError(err)
	}
	tc.namespacedJobIDs = [][2]string{}

	for _, ns := range tc.namespaceIDs {
		_, err := e2eutil.Command("nomad", "namespace", "delete", ns)
		f.NoError(err)
	}
	tc.namespaceIDs = []string{}

	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

// TestScalingBasic performs basic scaling e2e tests within a single namespace.
func (tc *ScalingE2ETest) TestScalingBasic(f *framework.F) {
	defaultNS := "default"

	// Register a job with a scaling policy. The group doesn't include the
	// count parameter, therefore Nomad should dynamically set this value to
	// the policy min.
	jobID := "test-scaling-" + uuid.Generate()[0:8]
	f.NoError(e2eutil.Register(jobID, "scaling/input/namespace_default_1.nomad"))
	tc.namespacedJobIDs = append(tc.namespacedJobIDs, [2]string{defaultNS, jobID})
	f.NoError(e2eutil.WaitForAllocStatusExpected(jobID, defaultNS, []string{"running", "running"}),
		"job should be running with 2 allocs")

	// Ensure we wait for the deployment to finish, otherwise scaling will
	// fail.
	f.NoError(e2eutil.WaitForLastDeploymentStatus(jobID, defaultNS, "successful", nil))

	// Simple scaling action.
	testMeta := map[string]interface{}{"scaling-e2e-test": "value"}
	scaleResp, _, err := tc.Nomad().Jobs().Scale(
		jobID, "horizontally_scalable", pointer.Of(3),
		"Nomad e2e testing", false, testMeta, nil)
	f.NoError(err)
	f.NotEmpty(scaleResp.EvalID)
	f.NoError(e2eutil.WaitForAllocStatusExpected(jobID, defaultNS, []string{"running", "running", "running"}),
		"job should be running with 3 allocs")

	// Ensure we wait for the deployment to finish, otherwise scaling will
	// fail for this reason.
	f.NoError(e2eutil.WaitForLastDeploymentStatus(jobID, defaultNS, "successful", nil))

	// Attempt break break the policy min/max parameters.
	_, _, err = tc.Nomad().Jobs().Scale(
		jobID, "horizontally_scalable", pointer.Of(4),
		"Nomad e2e testing", false, nil, nil)
	f.Error(err)
	_, _, err = tc.Nomad().Jobs().Scale(
		jobID, "horizontally_scalable", pointer.Of(1),
		"Nomad e2e testing", false, nil, nil)
	f.Error(err)

	// Check the scaling events.
	statusResp, _, err := tc.Nomad().Jobs().ScaleStatus(jobID, nil)
	f.NoError(err)
	f.Len(statusResp.TaskGroups["horizontally_scalable"].Events, 1)
	f.Equal(testMeta, statusResp.TaskGroups["horizontally_scalable"].Events[0].Meta)

	// Remove the job.
	_, _, err = tc.Nomad().Jobs().Deregister(jobID, true, nil)
	f.NoError(err)
	f.NoError(tc.Nomad().System().GarbageCollect())
	tc.namespacedJobIDs = [][2]string{}

	// Attempt job registrations where the group count violates the policy
	// min/max parameters.
	f.Error(e2eutil.Register(jobID, "scaling/input/namespace_default_2.nomad"))
	f.Error(e2eutil.Register(jobID, "scaling/input/namespace_default_3.nomad"))
}

// TestScalingNamespaces runs tests to ensure the job scaling endpoint adheres
// to Nomad's basic namespace principles.
func (tc *ScalingE2ETest) TestScalingNamespaces(f *framework.F) {

	defaultNS := "default"
	ANS := "NamespaceA"

	// Create our non-default namespace.
	_, err := e2eutil.Command("nomad", "namespace", "apply", ANS)
	f.NoError(err, "could not create namespace")
	tc.namespaceIDs = append(tc.namespaceIDs, ANS)

	defaultJobID := "test-scaling-default-" + uuid.Generate()[0:8]
	aJobID := "test-scaling-a-" + uuid.Generate()[0:8]

	// Register and wait for the job deployments to succeed.
	f.NoError(e2eutil.Register(defaultJobID, "scaling/input/namespace_default_1.nomad"))
	f.NoError(e2eutil.Register(aJobID, "scaling/input/namespace_a_1.nomad"))
	f.NoError(e2eutil.WaitForLastDeploymentStatus(defaultJobID, defaultNS, "successful", nil))
	f.NoError(e2eutil.WaitForLastDeploymentStatus(aJobID, ANS, "successful", nil))

	tc.namespacedJobIDs = append(tc.namespacedJobIDs, [2]string{defaultNS, defaultJobID})
	tc.namespacedJobIDs = append(tc.namespacedJobIDs, [2]string{ANS, aJobID})

	// Setup the WriteOptions for each namespace.
	defaultWriteOpts := api.WriteOptions{Namespace: defaultNS}
	aWriteOpts := api.WriteOptions{Namespace: ANS}

	// We shouldn't be able to trigger scaling across the namespace boundary.
	_, _, err = tc.Nomad().Jobs().Scale(
		defaultJobID, "horizontally_scalable", pointer.Of(3),
		"Nomad e2e testing", false, nil, &aWriteOpts)
	f.Error(err)
	_, _, err = tc.Nomad().Jobs().Scale(
		aJobID, "horizontally_scalable", pointer.Of(3),
		"Nomad e2e testing", false, nil, &defaultWriteOpts)
	f.Error(err)

	// We should be able to trigger scaling when using the correct namespace,
	// duh.
	_, _, err = tc.Nomad().Jobs().Scale(
		defaultJobID, "horizontally_scalable", pointer.Of(3),
		"Nomad e2e testing", false, nil, &defaultWriteOpts)
	f.NoError(err)
	_, _, err = tc.Nomad().Jobs().Scale(
		aJobID, "horizontally_scalable", pointer.Of(3),
		"Nomad e2e testing", false, nil, &aWriteOpts)
	f.NoError(err)
}

// TestScalingBasic performs basic scaling e2e tests within a single namespace using
// using a SystemScheduler.
func (tc *ScalingE2ETest) TestScalingBasicWithSystemSchedule(f *framework.F) {
	t := f.T()
	nomadClient := tc.Nomad()

	// Register a system job with a scaling policy without a group count, it should
	// default to 1 per node.

	jobID := "test-scaling-" + uuid.Generate()[0:8]
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "scaling/input/namespace_default_system.nomad", jobID, "")

	jobs := nomadClient.Jobs()
	initialAllocs, _, err := jobs.Allocations(jobID, true, nil)
	f.NoError(err)

	nodeStubList, _, err := nomadClient.Nodes().List(&api.QueryOptions{Namespace: "default"})
	f.NoError(err)

	// A system job will spawn an allocation per node, we need to know how many nodes
	// there are to know how many allocations to expect.
	numberOfNodes := len(nodeStubList)

	f.Equal(numberOfNodes, len(initialAllocs))
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(initialAllocs)

	// Wait for allocations to get past initial pending state
	e2eutil.WaitForAllocsNotPending(t, nomadClient, allocIDs)

	// Try to scale beyond 1
	testMeta := map[string]interface{}{"scaling-e2e-test": "value"}
	scaleResp, _, err := tc.Nomad().Jobs().Scale(jobID, "system_job_group", pointer.Of(3),
		"Nomad e2e testing", false, testMeta, nil)

	f.Error(err)
	f.Nil(scaleResp)

	// The same allocs should be running.
	jobs = nomadClient.Jobs()
	allocs1, _, err := jobs.Allocations(jobID, true, nil)
	f.NoError(err)

	f.Equal(len(initialAllocs), len(allocs1))

	for i, a := range allocs1 {
		f.Equal(a.ID, initialAllocs[i].ID)
	}

	// Scale down to 0
	testMeta = map[string]interface{}{"scaling-e2e-test": "value"}
	scaleResp, _, err = tc.Nomad().Jobs().Scale(jobID, "system_job_group", pointer.Of(0),
		"Nomad e2e testing", false, testMeta, nil)
	f.NoError(err)
	f.NotEmpty(scaleResp.EvalID)

	// Assert job is still up but no allocs are running
	stopedAllocs, _, err := jobs.Allocations(jobID, false, nil)
	f.NoError(err)

	f.Equal(numberOfNodes, len(filterAllocsByDesiredStatus(structs.AllocDesiredStatusStop, stopedAllocs)))
	f.Equal(numberOfNodes, len(stopedAllocs))

	// Scale up to 1 again
	testMeta = map[string]interface{}{"scaling-e2e-test": "value"}
	scaleResp, _, err = tc.Nomad().Jobs().Scale(jobID, "system_job_group", pointer.Of(1),
		"Nomad e2e testing", false, testMeta, nil)
	f.NoError(err)
	f.NotEmpty(scaleResp.EvalID)

	// Wait for new allocation to get past initial pending state
	e2eutil.WaitForAllocsNotPending(t, nomadClient, allocIDs)

	// Assert job is still running and there is a running allocation again
	allocs, _, err := jobs.Allocations(jobID, true, nil)
	f.NoError(err)
	f.Equal(numberOfNodes*2, len(allocs))

	f.Equal(numberOfNodes, len(filterAllocsByDesiredStatus(structs.AllocDesiredStatusStop, allocs)))
	f.Equal(numberOfNodes, len(filterAllocsByDesiredStatus(structs.AllocDesiredStatusRun, allocs)))

	// Remove the job.
	_, _, err = tc.Nomad().Jobs().Deregister(jobID, true, nil)
	f.NoError(err)
	f.NoError(tc.Nomad().System().GarbageCollect())
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
