// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/kr/pretty"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"

	"github.com/hashicorp/nomad/nomad/structs"
)

type testFn func() (bool, error)
type errorFn func(error)

func Wait(t *testing.T, test testFn) {
	t.Helper()
	retries := 500 * TestMultiplier()
	warn := int64(float64(retries) * 0.75)
	for tries := retries; tries > 0; {
		time.Sleep(10 * time.Millisecond)
		tries--

		success, err := test()
		if success {
			return
		}

		switch tries {
		case 0:
			if err == nil {
				t.Fatalf("timeout waiting for test function to succeed (you should probably return a helpful error instead of nil!)")
			} else {
				t.Fatalf("timeout: %v", err)
			}
		case warn:
			pc, _, _, _ := runtime.Caller(1)
			f := runtime.FuncForPC(pc)
			t.Logf("%d/%d retries reached for %s (err=%v)", warn, retries, f.Name(), err)
		}

	}
}

func WaitForResult(test testFn, error errorFn) {
	WaitForResultRetries(500*TestMultiplier(), test, error)
}

func WaitForResultRetries(retries int64, test testFn, error errorFn) {
	for retries > 0 {
		time.Sleep(10 * time.Millisecond)
		retries--

		success, err := test()
		if success {
			return
		}

		if retries == 0 {
			error(err)
		}
	}
}

// WaitForResultUntil waits the duration for the test to pass.
// Otherwise error is called after the deadline expires.
func WaitForResultUntil(until time.Duration, test testFn, errorFunc errorFn) {
	var success bool
	var err error
	deadline := time.Now().Add(until)
	for time.Now().Before(deadline) {
		success, err = test()
		if success {
			return
		}
		// Sleep some arbitrary fraction of the deadline
		time.Sleep(until / 30)
	}
	errorFunc(err)
}

// AssertUntil asserts the test function passes throughout the given duration.
// Otherwise error is called on failure.
func AssertUntil(until time.Duration, test testFn, error errorFn) {
	deadline := time.Now().Add(until)
	for time.Now().Before(deadline) {
		success, err := test()
		if !success {
			error(err)
			return
		}
		// Sleep some arbitrary fraction of the deadline
		time.Sleep(until / 30)
	}
}

// TestMultiplier returns a multiplier for retries and waits given environment
// the tests are being run under.
func TestMultiplier() int64 {
	if IsCI() {
		return 4
	}

	return 1
}

// Timeout takes the desired timeout and increases it if running in Travis
func Timeout(original time.Duration) time.Duration {
	return original * time.Duration(TestMultiplier())
}

func IsCI() bool {
	_, ok := os.LookupEnv("CI")
	return ok
}

func IsTravis() bool {
	_, ok := os.LookupEnv("TRAVIS")
	return ok
}

func IsAppVeyor() bool {
	_, ok := os.LookupEnv("APPVEYOR")
	return ok
}

type rpcFn func(string, interface{}, interface{}) error

// WaitForLeader blocks until a leader is elected.
func WaitForLeader(t testing.TB, rpc rpcFn) {
	t.Helper()
	WaitForResult(func() (bool, error) {
		args := &structs.GenericRequest{}
		var leader string
		err := rpc("Status.Leader", args, &leader)
		return leader != "", err
	}, func(err error) {
		t.Fatalf("failed to find leader: %v", err)
	})
}

// WaitForLeaders blocks until each rpcs knows the leader.
func WaitForLeaders(t testing.TB, rpcs ...rpcFn) string {
	t.Helper()

	var leader string
	for i := 0; i < len(rpcs); i++ {
		ok := func() (bool, error) {
			leader = ""
			args := &structs.GenericRequest{}
			err := rpcs[i]("Status.Leader", args, &leader)
			return leader != "", err
		}
		must.Wait(t, wait.InitialSuccess(
			wait.TestFunc(ok),
			wait.Timeout(10*time.Second),
			wait.Gap(1*time.Second),
		))
	}

	return leader
}

// WaitForClient blocks until the client can be found
func WaitForClient(t testing.TB, rpc rpcFn, nodeID string, region string) {
	t.Helper()
	WaitForClientStatus(t, rpc, nodeID, region, structs.NodeStatusReady)
}

// WaitForClientStatus blocks until the client is in the expected status.
func WaitForClientStatus(t testing.TB, rpc rpcFn, nodeID string, region string, status string) {
	t.Helper()

	if region == "" {
		region = "global"
	}
	WaitForResult(func() (bool, error) {
		req := structs.NodeSpecificRequest{
			NodeID:       nodeID,
			QueryOptions: structs.QueryOptions{Region: region},
		}
		var out structs.SingleNodeResponse

		err := rpc("Node.GetNode", &req, &out)
		if err != nil {
			return false, err
		}
		if out.Node == nil {
			return false, fmt.Errorf("node not found")
		}
		if out.Node.Status != status {
			return false, fmt.Errorf("node is %s, not %s", out.Node.Status, status)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("failed to wait for node staus: %v", err)
	})

	t.Logf("[TEST] Client for test %s %s, id: %s, region: %s", t.Name(), status, nodeID, region)
}

// WaitForVotingMembers blocks until autopilot promotes all server peers
// to be voting members.
//
// Useful for tests that change cluster topology (e.g. kill a node)
// that should wait until cluster is stable.
func WaitForVotingMembers(t testing.TB, rpc rpcFn, nPeers int) {
	WaitForResult(func() (bool, error) {
		args := &structs.GenericRequest{}
		args.AllowStale = true
		args.Region = "global"
		args.Namespace = structs.DefaultNamespace
		resp := structs.RaftConfigurationResponse{}
		err := rpc("Operator.RaftGetConfiguration", args, &resp)
		if err != nil {
			return false, fmt.Errorf("failed to query raft: %v", err)
		}

		if len(resp.Servers) != nPeers {
			return false, fmt.Errorf("expected %d peers found %d", nPeers, len(resp.Servers))
		}

		for _, s := range resp.Servers {
			if !s.Voter {
				return false, fmt.Errorf("found nonvoting server: %v", s)
			}
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("failed to wait until voting members: %v", err)
	})
}

// RegisterJobWithToken registers a job and uses the job's Region and Namespace.
func RegisterJobWithToken(t testing.TB, rpc rpcFn, job *structs.Job, token string) {
	t.Helper()
	WaitForResult(func() (bool, error) {
		args := &structs.JobRegisterRequest{}
		args.Job = job
		args.WriteRequest.Region = job.Region
		args.AuthToken = token
		args.Namespace = job.Namespace
		var jobResp structs.JobRegisterResponse
		err := rpc("Job.Register", args, &jobResp)
		return err == nil, fmt.Errorf("Job.Register error: %v", err)
	}, func(err error) {
		t.Fatalf("error registering job: %v", err)
	})

	t.Logf("Job %q registered", job.ID)
}

func RegisterJob(t testing.TB, rpc rpcFn, job *structs.Job) {
	RegisterJobWithToken(t, rpc, job, "")
}

func WaitForRunningWithToken(t testing.TB, rpc rpcFn, job *structs.Job, token string) []*structs.AllocListStub {
	RegisterJobWithToken(t, rpc, job, token)

	var resp structs.JobAllocationsResponse

	// This can be quite slow if the job has expensive setup such as
	// downloading large artifacts or creating a chroot.
	WaitForResultRetries(2000*TestMultiplier(), func() (bool, error) {
		args := &structs.JobSpecificRequest{}
		args.JobID = job.ID
		args.QueryOptions.Region = job.Region
		args.AuthToken = token
		args.Namespace = job.Namespace
		err := rpc("Job.Allocations", args, &resp)
		if err != nil {
			return false, fmt.Errorf("Job.Allocations error: %v", err)
		}

		if len(resp.Allocations) == 0 {
			evals := structs.JobEvaluationsResponse{}
			must.NoError(t, rpc("Job.Evaluations", args, &evals), must.Sprintf("error looking up evals"))
			return false, fmt.Errorf("0 allocations; evals: %s", pretty.Sprint(evals.Evaluations))
		}

		for _, alloc := range resp.Allocations {
			if alloc.ClientStatus == structs.AllocClientStatusPending {
				return false, fmt.Errorf("alloc not running: id=%v tg=%v status=%v",
					alloc.ID, alloc.TaskGroup, alloc.ClientStatus)
			}
		}

		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})

	return resp.Allocations
}

// WaitForRunning runs a job and blocks until all allocs are out of pending.
func WaitForRunning(t testing.TB, rpc rpcFn, job *structs.Job) []*structs.AllocListStub {
	return WaitForRunningWithToken(t, rpc, job, "")
}

// WaitforJobAllocStatus blocks until the ClientStatus of allocations for a job
// match the expected map of <ClientStatus>: <count>.
func WaitForJobAllocStatus(t testing.TB, rpc rpcFn, job *structs.Job, allocStatus map[string]int) {
	t.Helper()
	WaitForJobAllocStatusWithToken(t, rpc, job, allocStatus, "")
}

// WaitForJobAllocStatusWithToken behaves the same way as WaitForJobAllocStatus
// but is used for clusters with ACL enabled.
func WaitForJobAllocStatusWithToken(t testing.TB, rpc rpcFn, job *structs.Job, allocStatus map[string]int, token string) []*structs.AllocListStub {
	t.Helper()

	var allocs []*structs.AllocListStub
	WaitForResultRetries(2000*TestMultiplier(), func() (bool, error) {
		args := &structs.JobSpecificRequest{
			JobID: job.ID,
			QueryOptions: structs.QueryOptions{
				AuthToken: token,
				Namespace: job.Namespace,
				Region:    job.Region,
			},
		}

		var resp structs.JobAllocationsResponse
		err := rpc("Job.Allocations", args, &resp)
		if err != nil {
			return false, fmt.Errorf("Job.Allocations error: %v", err)
		}

		if len(resp.Allocations) == 0 {
			evals := structs.JobEvaluationsResponse{}
			must.NoError(t, rpc("Job.Evaluations", args, &evals), must.Sprintf("error looking up evals"))
			return false, fmt.Errorf("0 allocations; evals: %s", pretty.Sprint(evals.Evaluations))
		}

		allocs = resp.Allocations

		got := map[string]int{}
		for _, alloc := range resp.Allocations {
			got[alloc.ClientStatus]++
		}
		if diff := cmp.Diff(allocStatus, got); diff != "" {
			return false, fmt.Errorf("alloc status mismatch (-want +got):\n%s", diff)
		}
		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})

	return allocs
}

// WaitForFiles blocks until all the files in the slice are present
func WaitForFiles(t testing.TB, files []string) {
	WaitForResult(func() (bool, error) {
		return FilesExist(files)
	}, func(err error) {
		t.Fatalf("missing expected files: %v", err)
	})
}

// WaitForFilesUntil blocks until duration or all the files in the slice are present
func WaitForFilesUntil(t testing.TB, files []string, until time.Duration) {
	WaitForResultUntil(until, func() (bool, error) {
		return FilesExist(files)
	}, func(err error) {
		t.Fatalf("missing expected files: %v", err)
	})
}

// FilesExist verifies all files in the slice are present
func FilesExist(files []string) (bool, error) {
	for _, f := range files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			return false, fmt.Errorf("expected file not found: %v", f)
		}
	}
	return true, nil
}
