// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package widmgr_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestWIDMgr(t *testing.T) {
	ci.Parallel(t)

	// Create a mixed ta
	ta := agent.NewTestAgent(t, "widtest", func(c *agent.Config) {
		c.Server.Enabled = true
		c.Server.NumSchedulers = pointer.Of(1)
		c.Client.Enabled = true
	})
	t.Cleanup(ta.Shutdown)

	mgr := widmgr.New(widmgr.Config{
		NodeSecret: uuid.Generate(), // not checked when ACLs disabled
		Region:     "global",
		RPC:        ta,
	})

	_, err := mgr.SignIdentities(1, nil)
	must.ErrorContains(t, err, "no identities requested")

	_, err = mgr.SignIdentities(1, []*structs.WorkloadIdentityRequest{
		{
			AllocID:      uuid.Generate(),
			TaskName:     "web",
			IdentityName: "foo",
		},
	})
	must.ErrorContains(t, err, "rejected")

	// Register a job with 3 identities (but only 2 that need signing)
	job := mock.MinJob()
	job.TaskGroups[0].Tasks[0].Identity = &structs.WorkloadIdentity{
		Env: true,
	}
	job.TaskGroups[0].Tasks[0].Identities = []*structs.WorkloadIdentity{
		{
			Name:     "consul",
			Audience: []string{"a", "b"},
			Env:      true,
		},
		{
			Name: "vault",
			File: true,
		},
	}
	job.Canonicalize()

	testutil.RegisterJob(t, ta.RPC, job)

	var allocs []*structs.AllocListStub
	testutil.WaitForResult(func() (bool, error) {
		args := &structs.JobSpecificRequest{}
		args.JobID = job.ID
		args.QueryOptions.Region = job.Region
		args.Namespace = job.Namespace
		var resp structs.JobAllocationsResponse
		err := ta.RPC("Job.Allocations", args, &resp)
		if err != nil {
			return false, fmt.Errorf("Job.Allocations error: %v", err)
		}

		if len(resp.Allocations) == 0 {
			return false, fmt.Errorf("no allocs")
		}
		allocs = resp.Allocations
		return len(allocs) == 1, fmt.Errorf("unexpected number of allocs: %d", len(allocs))
	}, func(err error) {
		must.NoError(t, err)
	})
	must.Len(t, 1, allocs)

	// Get signed identites for alloc
	widreqs := []*structs.WorkloadIdentityRequest{
		{
			AllocID:      allocs[0].ID,
			TaskName:     job.TaskGroups[0].Tasks[0].Name,
			IdentityName: "consul",
		},
		{
			AllocID:      allocs[0].ID,
			TaskName:     job.TaskGroups[0].Tasks[0].Name,
			IdentityName: "vault",
		},
	}

	swids, err := mgr.SignIdentities(allocs[0].CreateIndex, widreqs)
	must.NoError(t, err)
	must.Len(t, 2, swids)
	must.Eq(t, *widreqs[0], swids[0].WorkloadIdentityRequest)
	must.StrContains(t, swids[0].JWT, ".")
	must.Eq(t, *widreqs[1], swids[1].WorkloadIdentityRequest)
	must.StrContains(t, swids[1].JWT, ".")
}
