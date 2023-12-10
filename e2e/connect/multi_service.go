// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"fmt"
	"strings"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

// TestMultiServiceConnect tests running multiple envoy sidecars in the same allocation.
func (tc *ConnectE2ETest) TestMultiServiceConnect(f *framework.F) {
	t := f.T()
	uuid := uuid.Generate()
	jobID := "connect" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobID)
	jobapi := tc.Nomad().Jobs()

	job, err := jobspec.ParseFile("connect/input/multi-service.nomad")
	require.NoError(t, err)
	job.ID = &jobID

	resp, _, err := jobapi.Register(job, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Zero(t, resp.Warnings)

EVAL:
	qopts := &api.QueryOptions{
		WaitIndex: resp.EvalCreateIndex,
	}
	evalapi := tc.Nomad().Evaluations()
	eval, qmeta, err := evalapi.Info(resp.EvalID, qopts)
	require.NoError(t, err)
	qopts.WaitIndex = qmeta.LastIndex

	switch eval.Status {
	case "pending":
		goto EVAL
	case "complete":
		// Ok!
	case "failed", "canceled", "blocked":
		require.Failf(t, "expected complete status", "eval %s\n%s", eval.Status, pretty.Sprint(eval))
	default:
		require.Failf(t, "expected complete status", "unknown eval status: %s\n%s", eval.Status, pretty.Sprint(eval))
	}

	// Assert there were 0 placement failures
	require.Zero(t, eval.FailedTGAllocs, pretty.Sprint(eval.FailedTGAllocs))
	require.Len(t, eval.QueuedAllocations, 1, pretty.Sprint(eval.QueuedAllocations))

	// Assert allocs are running
	for i := 0; i < 20; i++ {
		allocs, qmeta, err := evalapi.Allocations(eval.ID, qopts)
		require.NoError(t, err)
		require.Len(t, allocs, 1)
		qopts.WaitIndex = qmeta.LastIndex

		running := 0
		for _, alloc := range allocs {
			switch alloc.ClientStatus {
			case "running":
				running++
			case "pending":
				// keep trying
			default:
				require.Failf(t, "alloc failed", "alloc: %s", pretty.Sprint(alloc))
			}
		}

		if running == len(allocs) {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	allocs, _, err := evalapi.Allocations(eval.ID, qopts)
	require.NoError(t, err)
	allocIDs := make(map[string]bool, 1)
	for _, a := range allocs {
		if a.ClientStatus != "running" || a.DesiredStatus != "run" {
			require.Failf(t, "expected running status", "alloc %s (%s) terminal; client=%s desired=%s", a.TaskGroup, a.ID, a.ClientStatus, a.DesiredStatus)
		}
		allocIDs[a.ID] = true
	}

	// Check Consul service health
	agentapi := tc.Consul().Agent()

	failing := map[string]*consulapi.AgentCheck{}
	testutil.WaitForResultRetries(60, func() (bool, error) {
		defer time.Sleep(time.Second)

		checks, err := agentapi.Checks()
		require.NoError(t, err)

		// Filter out checks for other services
		for cid, check := range checks {
			found := false
			for allocID := range allocIDs {
				if strings.Contains(check.ServiceID, allocID) {
					found = true
					break
				}
			}

			if !found {
				delete(checks, cid)
			}
		}

		// Ensure checks are all passing
		failing = map[string]*consulapi.AgentCheck{}
		for _, check := range checks {
			if check.Status != "passing" {
				failing[check.CheckID] = check
				break
			}
		}

		if len(failing) == 0 {
			return true, nil
		}

		t.Logf("still %d checks not passing", len(failing))
		return false, fmt.Errorf("checks are not passing %v %v", len(failing), pretty.Sprint(failing))
	}, func(e error) {
		require.NoError(t, err)
	})

	require.Len(t, failing, 0, pretty.Sprint(failing))
}
