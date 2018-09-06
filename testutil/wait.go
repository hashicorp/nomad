package testutil

import (
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/go-testing-interface"
)

const (
	// TravisRunEnv is an environment variable that is set if being run by
	// Travis.
	TravisRunEnv = "CI"
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
	if IsTravis() {
		return 4
	}

	return 1
}

// Timeout takes the desired timeout and increases it if running in Travis
func Timeout(original time.Duration) time.Duration {
	return original * time.Duration(TestMultiplier())
}

func IsTravis() bool {
	_, ok := os.LookupEnv(TravisRunEnv)
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

// WaitForRunning runs a job and blocks until it is running.
func WaitForRunning(t testing.T, rpc rpcFn, job *structs.Job) {
	registered := false
	WaitForResult(func() (bool, error) {
		if !registered {
			args := &structs.JobRegisterRequest{}
			args.Job = job
			args.WriteRequest.Region = "global"
			var jobResp structs.JobRegisterResponse
			err := rpc("Job.Register", args, &jobResp)
			if err != nil {
				return false, fmt.Errorf("Job.Register error: %v", err)
			}

			// Only register once
			registered = true
		}

		args := &structs.JobSummaryRequest{}
		args.JobID = job.ID
		args.QueryOptions.Region = "global"
		var resp structs.JobSummaryResponse
		err := rpc("Job.Summary", args, &resp)
		if err != nil {
			return false, fmt.Errorf("Job.Summary error: %v", err)
		}

		tgs := len(job.TaskGroups)
		summaries := len(resp.JobSummary.Summary)
		if tgs != summaries {
			return false, fmt.Errorf("task_groups=%d summaries=%d", tgs, summaries)
		}

		for tg, summary := range resp.JobSummary.Summary {
			if summary.Running == 0 {
				return false, fmt.Errorf("task_group=%s %#v", tg, resp.JobSummary.Summary)
			}
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("job not running: %v", err)
	})
}
