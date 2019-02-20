package testutil

import (
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	testing "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

type testFn func() (bool, error)
type errorFn func(error)

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
func WaitForLeader(t testing.T, rpc rpcFn) {
	WaitForResult(func() (bool, error) {
		args := &structs.GenericRequest{}
		var leader string
		err := rpc("Status.Leader", args, &leader)
		return leader != "", err
	}, func(err error) {
		t.Fatalf("failed to find leader: %v", err)
	})
}

func RegisterJob(t testing.T, rpc rpcFn, job *structs.Job) {
	WaitForResult(func() (bool, error) {
		args := &structs.JobRegisterRequest{}
		args.Job = job
		args.WriteRequest.Region = "global"
		var jobResp structs.JobRegisterResponse
		err := rpc("Job.Register", args, &jobResp)
		return err == nil, fmt.Errorf("Job.Register error: %v", err)
	}, func(err error) {
		t.Fatalf("error registering job: %v", err)
	})

	t.Logf("Job %q registered", job.ID)
}

// WaitForRunning runs a job and blocks until all allocs are out of pending.
func WaitForRunning(t testing.T, rpc rpcFn, job *structs.Job) []*structs.AllocListStub {
	RegisterJob(t, rpc, job)

	var resp structs.JobAllocationsResponse

	WaitForResult(func() (bool, error) {
		args := &structs.JobSpecificRequest{}
		args.JobID = job.ID
		args.QueryOptions.Region = "global"
		err := rpc("Job.Allocations", args, &resp)
		if err != nil {
			return false, fmt.Errorf("Job.Allocations error: %v", err)
		}

		if len(resp.Allocations) == 0 {
			return false, fmt.Errorf("0 allocations")
		}

		for _, alloc := range resp.Allocations {
			if alloc.ClientStatus == structs.AllocClientStatusPending {
				return false, fmt.Errorf("alloc not running: id=%v tg=%v status=%v",
					alloc.ID, alloc.TaskGroup, alloc.ClientStatus)
			}
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	return resp.Allocations
}
