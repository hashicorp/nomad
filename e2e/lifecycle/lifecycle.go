// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lifecycle

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

type LifecycleE2ETest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Lifecycle",
		CanRunLocal: true,
		Cases:       []framework.TestCase{new(LifecycleE2ETest)},
	})
}

// BeforeAll ensures the cluster has leader and at least 1 client node in a
// ready state before running tests.
func (tc *LifecycleE2ETest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

// TestBatchJob runs a batch job with prestart and poststop hooks
func (tc *LifecycleE2ETest) TestBatchJob(f *framework.F) {
	t := f.T()
	require := require.New(t)
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobID := "lifecycle-" + uuid[0:8]
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "lifecycle/inputs/batch.nomad", jobID, "")
	require.Equal(1, len(allocs))
	allocID := allocs[0].ID

	// wait for the job to stop and assert we stopped successfully, not failed
	e2eutil.WaitForAllocStopped(t, nomadClient, allocID)
	alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
	require.NoError(err)
	require.Equal(structs.AllocClientStatusComplete, alloc.ClientStatus)

	// assert the files were written as expected
	afi, _, err := nomadClient.AllocFS().List(alloc, "alloc", nil)
	require.NoError(err)
	expected := map[string]bool{
		"init-ran": true, "main-ran": true, "poststart-ran": true, "poststop-ran": true,
		"init-running": false, "main-running": false, "poststart-running": false}
	got := checkFiles(expected, afi)
	require.Equal(expected, got)
}

// TestServiceJob runs a service job with prestart and poststop hooks
func (tc *LifecycleE2ETest) TestServiceJob(f *framework.F) {
	t := f.T()
	require := require.New(t)
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobID := "lifecycle-" + uuid[0:8]
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "lifecycle/inputs/service.nomad", jobID, "")
	require.Equal(1, len(allocs))
	allocID := allocs[0].ID

	//e2eutil.WaitForAllocRunning(t, nomadClient, allocID)
	testutil.WaitForResult(func() (bool, error) {
		alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
		if err != nil {
			return false, err
		}

		if alloc.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("expected status running, but was: %s", alloc.ClientStatus)
		}

		if alloc.TaskStates["poststart"].FinishedAt.IsZero() {
			return false, fmt.Errorf("poststart task hasn't started")
		}

		afi, _, err := nomadClient.AllocFS().List(alloc, "alloc", nil)
		if err != nil {
			return false, err
		}
		expected := map[string]bool{
			"main-checked": true}
		got := checkFiles(expected, afi)
		if !got["main-checked"] {
			return false, fmt.Errorf("main-checked file has not been written")
		}

		return true, nil
	}, func(err error) {
		require.NoError(err, "failed to wait on alloc")
	})

	alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
	require.NoError(err)

	require.False(alloc.TaskStates["poststart"].Failed)

	// stop the job
	_, _, err = nomadClient.Jobs().Deregister(jobID, false, nil)
	require.NoError(err)
	e2eutil.WaitForAllocStopped(t, nomadClient, allocID)

	require.False(alloc.TaskStates["poststop"].Failed)

	// assert the files were written as expected
	afi, _, err := nomadClient.AllocFS().List(alloc, "alloc", nil)
	require.NoError(err)
	expected := map[string]bool{
		"init-ran": true, "sidecar-ran": true, "main-ran": true, "poststart-ran": true, "poststop-ran": true,
		"poststart-started": true, "main-started": true, "poststop-started": true,
		"init-running": false, "poststart-running": false, "poststop-running": false,
		"main-checked": true}
	got := checkFiles(expected, afi)
	require.Equal(expected, got)
}

// checkFiles returns a map of whether the expected files were found
// in the file info response
func checkFiles(expected map[string]bool, got []*api.AllocFileInfo) map[string]bool {
	results := map[string]bool{}
	for expect := range expected {
		results[expect] = false
	}
	for _, file := range got {
		// there will be files unrelated to the test, so ignore those
		if _, ok := results[file.Name]; ok {
			results[file.Name] = true
		}
	}
	return results
}
