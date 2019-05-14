package e2eutil

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
)

// retries is used to control how many times to retry checking if the cluster has a leader yet
const retries = 500

func WaitForLeader(t *testing.T, nomadClient *api.Client) {
	statusAPI := nomadClient.Status()

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		leader, err := statusAPI.Leader()
		return leader != "", err
	}, func(err error) {
		t.Fatalf("failed to find leader: %v", err)
	})
}

// WaitForNodesReady waits until at least `nodes` number of nodes are ready or
// fails the test.
func WaitForNodesReady(t *testing.T, nomadClient *api.Client, nodes int) {
	nodesAPI := nomadClient.Nodes()

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		defer time.Sleep(time.Millisecond * 100)
		nodesList, _, err := nodesAPI.List(nil)
		if err != nil {
			return false, fmt.Errorf("error listing nodes: %v", err)
		}

		eligibleNodes := 0
		for _, node := range nodesList {
			if node.Status == "ready" {
				eligibleNodes++
			}
		}

		return eligibleNodes >= nodes, fmt.Errorf("only %d nodes ready (wanted at least %d)", eligibleNodes, nodes)
	}, func(err error) {
		t.Fatalf("failed to get enough ready nodes: %v", err)
	})
}

func RegisterAndWaitForAllocs(t *testing.T, nomadClient *api.Client, jobFile string, jobID string) []*api.AllocationListStub {
	// Parse job
	job, err := jobspec.ParseFile(jobFile)
	require := require.New(t)
	require.Nil(err)
	job.ID = helper.StringToPtr(jobID)

	g := NewGomegaWithT(t)

	// Register job
	jobs := nomadClient.Jobs()
	testutil.WaitForResult(func() (bool, error) {
		resp, _, err := jobs.Register(job, nil)
		if err != nil {
			return false, err
		}
		return resp.EvalID != "", fmt.Errorf("expected EvalID:%s", pretty.Sprint(resp))
	}, func(err error) {
		require.NoError(err)
	})

	// Wrap in retry to wait until placement
	g.Eventually(func() []*api.AllocationListStub {
		// Look for allocations
		allocs, _, _ := jobs.Allocations(*job.ID, false, nil)
		return allocs
	}, 30*time.Second, time.Second).ShouldNot(BeEmpty())

	allocs, _, err := jobs.Allocations(*job.ID, false, nil)
	require.Nil(err)
	return allocs
}

func WaitForAllocRunning(t *testing.T, nomadClient *api.Client, allocID string) {
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(time.Millisecond * 100)
		alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
		if err != nil {
			return false, err
		}

		return alloc.ClientStatus == structs.AllocClientStatusRunning, fmt.Errorf("expected status running, but was: %s", alloc.ClientStatus)
	}, func(err error) {
		t.Fatalf("failed to wait on alloc: %v", err)
	})
}
