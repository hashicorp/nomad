package e2eutil

import (
	"fmt"
	"os"
	"testing"
	"time"

	api "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/jobspec2"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
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
		require.NoError(t, err, "failed to find leader")
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
		require.NoError(t, err, "failed to get enough ready nodes")
	})
}

func stringToPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return helper.StringToPtr(s)
}

func Parse2(t *testing.T, jobFile string) (*api.Job, error) {
	f, err := os.Open(jobFile)
	require.NoError(t, err)
	return jobspec2.Parse(jobFile, f)
}

func RegisterAllocs(t *testing.T, nomadClient *api.Client, jobFile, jobID, cToken string) []*api.AllocationListStub {

	// Parse job
	job, err := Parse2(t, jobFile)
	require.NoError(t, err)

	// Set custom job ID (distinguish among tests)
	job.ID = helper.StringToPtr(jobID)

	// Set a Consul "operator" token for the job, if provided.
	job.ConsulToken = stringToPtrOrNil(cToken)

	// Register job
	var idx uint64
	jobs := nomadClient.Jobs()
	testutil.WaitForResult(func() (bool, error) {
		resp, meta, err := jobs.Register(job, nil)
		if err != nil {
			return false, err
		}
		idx = meta.LastIndex
		return resp.EvalID != "", fmt.Errorf("expected EvalID:%s", pretty.Sprint(resp))
	}, func(err error) {
		require.NoError(t, err)
	})

	allocs, _, err := jobs.Allocations(jobID, false, &api.QueryOptions{WaitIndex: idx})
	require.NoError(t, err)
	return allocs
}

// RegisterAndWaitForAllocs wraps RegisterAllocs but blocks until Evals
// successfully create Allocs.
func RegisterAndWaitForAllocs(t *testing.T, nomadClient *api.Client, jobFile, jobID, cToken string) []*api.AllocationListStub {
	jobs := nomadClient.Jobs()

	// Start allocations
	RegisterAllocs(t, nomadClient, jobFile, jobID, cToken)

	var err error
	allocs := []*api.AllocationListStub{}
	evals := []*api.Evaluation{}

	// Wrap in retry to wait until placement
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(time.Second)

		allocs, _, err = jobs.Allocations(jobID, false, nil)
		if len(allocs) == 0 {
			evals, _, err = nomadClient.Jobs().Evaluations(jobID, nil)
			return false, fmt.Errorf("no allocations for job %v", jobID)
		}

		return true, nil
	}, func(e error) {
		msg := fmt.Sprintf("allocations not placed for %s", jobID)
		for _, eval := range evals {
			msg += fmt.Sprintf("\n  %s - %s", eval.Status, eval.StatusDescription)
		}

		require.Fail(t, msg, "full evals: %v", pretty.Sprint(evals))
	})

	require.NoError(t, err) // we only care about the last error

	return allocs
}

func WaitForAllocRunning(t *testing.T, nomadClient *api.Client, allocID string) {
	t.Helper()

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(time.Millisecond * 100)
		alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
		if err != nil {
			return false, err
		}

		return alloc.ClientStatus == structs.AllocClientStatusRunning, fmt.Errorf("expected status running, but was: %s\n%v", alloc.ClientStatus, pretty.Sprint(alloc))
	}, func(err error) {
		require.NoError(t, err, "failed to wait on alloc")
	})
}

func WaitForAllocTaskRunning(t *testing.T, nomadClient *api.Client, allocID, task string) {
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(time.Millisecond * 100)
		alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
		if err != nil {
			return false, err
		}

		state := "n/a"
		if task := alloc.TaskStates[task]; task != nil {
			state = task.State
		}
		return state == structs.AllocClientStatusRunning, fmt.Errorf("expected status running, but was: %s", state)
	}, func(err error) {
		t.Fatalf("failed to wait on alloc: %v", err)
	})
}

func WaitForAllocsRunning(t *testing.T, nomadClient *api.Client, allocIDs []string) {
	for _, allocID := range allocIDs {
		WaitForAllocRunning(t, nomadClient, allocID)
	}
}

func WaitForAllocsNotPending(t *testing.T, nomadClient *api.Client, allocIDs []string) {
	for _, allocID := range allocIDs {
		WaitForAllocNotPending(t, nomadClient, allocID)
	}
}

func WaitForAllocNotPending(t *testing.T, nomadClient *api.Client, allocID string) {
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(time.Millisecond * 100)
		alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
		if err != nil {
			return false, err
		}

		return alloc.ClientStatus != structs.AllocClientStatusPending, fmt.Errorf("expected status not pending, but was: %s", alloc.ClientStatus)
	}, func(err error) {
		require.NoError(t, err, "failed to wait on alloc")
	})
}

// WaitForJobStopped stops a job and waits for all of its allocs to terminate.
func WaitForJobStopped(t *testing.T, nomadClient *api.Client, job string) {
	allocs, _, err := nomadClient.Jobs().Allocations(job, true, nil)
	require.NoError(t, err, "error getting allocations for job %q", job)
	ids := AllocIDsFromAllocationListStubs(allocs)
	_, _, err = nomadClient.Jobs().Deregister(job, true, nil)
	require.NoError(t, err, "error deregistering job %q", job)
	for _, id := range ids {
		WaitForAllocStopped(t, nomadClient, id)
	}
}

func WaitForAllocsStopped(t *testing.T, nomadClient *api.Client, allocIDs []string) {
	for _, allocID := range allocIDs {
		WaitForAllocStopped(t, nomadClient, allocID)
	}
}

func WaitForAllocStopped(t *testing.T, nomadClient *api.Client, allocID string) {
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(time.Millisecond * 100)
		alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
		if err != nil {
			return false, err
		}
		switch alloc.ClientStatus {
		case structs.AllocClientStatusComplete:
			return true, nil
		case structs.AllocClientStatusFailed:
			return true, nil
		case structs.AllocClientStatusLost:
			return true, nil
		default:
			return false, fmt.Errorf("expected stopped alloc, but was: %s",
				alloc.ClientStatus)
		}
	}, func(err error) {
		require.NoError(t, err, "failed to wait on alloc")
	})
}

func AllocIDsFromAllocationListStubs(allocs []*api.AllocationListStub) []string {
	allocIDs := make([]string, 0, len(allocs))
	for _, alloc := range allocs {
		allocIDs = append(allocIDs, alloc.ID)
	}
	return allocIDs
}

func DeploymentsForJob(t *testing.T, nomadClient *api.Client, jobID string) []*api.Deployment {
	ds, _, err := nomadClient.Deployments().List(nil)
	require.NoError(t, err)

	out := []*api.Deployment{}
	for _, d := range ds {
		if d.JobID == jobID {
			out = append(out, d)
		}
	}

	return out
}

func WaitForDeployment(t *testing.T, nomadClient *api.Client, deployID string, status string, statusDesc string) {
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(time.Millisecond * 100)
		deploy, _, err := nomadClient.Deployments().Info(deployID, nil)
		if err != nil {
			return false, err
		}

		if deploy.Status == status && deploy.StatusDescription == statusDesc {
			return true, nil
		}
		return false, fmt.Errorf("expected status %s \"%s\", but got: %s \"%s\"",
			status,
			statusDesc,
			deploy.Status,
			deploy.StatusDescription,
		)

	}, func(err error) {
		require.NoError(t, err, "failed to wait on deployment")
	})
}
