// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package batch_timeout

import (
	"fmt"
	"testing"
	"time"

	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/v2/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/v2/e2e/v3/jobs3"
	"github.com/hashicorp/nomad/v2/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

// testTimeout is the upper bound we allow for each sub-test. It must be long
// enough to cover: job registration → eval → alloc placement → the
// max_run_duration timer firing (5 s) → terminal state propagation.
const testTimeout = 60 * time.Second

func TestBatchJobMaxRunDuration(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
	)

	t.Run("testBatchTimesOut", testBatchJobTimesOut)
	t.Run("testBatchCompletesBeforeTimeout", testBatchJobCompletesBeforeTimeout)
	t.Run("testBatchTimeoutSkipsPoststop", testBatchJobTimeoutSkipsPoststop)
	t.Run("testSysbatchTimesOut", testSysbatchJobTimesOut)
}

// testBatchJobTimesOut verifies that a batch job with max_run_duration is
// killed after the configured deadline and that the resulting allocation:
//   - has ClientStatus == "complete" (not "failed")
//   - has ClientDescription == AllocTimeoutReasonMaxRunDuration
func testBatchJobTimesOut(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/batch_timeout_basic.hcl",
		jobs3.WaitComplete("group"),
		jobs3.Timeout(testTimeout),
	)
	t.Cleanup(cleanup)

	allocs := job.Allocs()
	must.Len(t, 1, allocs,
		must.Sprint("expected exactly one allocation for this batch job"))

	alloc := allocs[0]
	must.Eq(t, nomadapi.AllocClientStatusComplete, alloc.ClientStatus,
		must.Sprint("allocation timed out by max_run_duration must be 'complete', not 'failed'"))
	must.Eq(t, structs.AllocTimeoutReasonMaxRunDuration, alloc.ClientDescription,
		must.Sprint("allocation description must indicate a max_run_duration timeout"))
}

// testBatchJobCompletesBeforeTimeout verifies that a batch job whose task
// finishes before the deadline completes normally; the timeout description must
// not appear on the allocation.
func testBatchJobCompletesBeforeTimeout(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/batch_timeout_completes.hcl",
		jobs3.WaitComplete("group"),
		jobs3.Timeout(testTimeout),
	)
	t.Cleanup(cleanup)

	allocs := job.Allocs()
	must.Len(t, 1, allocs,
		must.Sprint("expected exactly one allocation for this batch job"))

	alloc := allocs[0]
	must.Eq(t, nomadapi.AllocClientStatusComplete, alloc.ClientStatus,
		must.Sprint("allocation must be 'complete' after finishing normally"))
	must.NotEq(t, structs.AllocTimeoutReasonMaxRunDuration, alloc.ClientDescription,
		must.Sprint("allocation description must NOT indicate a timeout when job finishes naturally"))
}

// testBatchJobTimeoutSkipsPoststop verifies that when a batch job is terminated
// by max_run_duration, its poststop lifecycle task is not started.
func testBatchJobTimeoutSkipsPoststop(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/batch_timeout_poststop.hcl",
		jobs3.WaitComplete("group"),
		jobs3.Timeout(testTimeout),
	)
	t.Cleanup(cleanup)

	allocs := job.Allocs()
	must.Len(t, 1, allocs,
		must.Sprint("expected exactly one allocation for this batch job"))

	alloc := allocs[0]

	// Confirm the allocation was indeed terminated by the deadline.
	must.Eq(t, nomadapi.AllocClientStatusComplete, alloc.ClientStatus,
		must.Sprint("timed-out allocation must be 'complete'"))
	must.Eq(t, structs.AllocTimeoutReasonMaxRunDuration, alloc.ClientDescription,
		must.Sprint("timed-out allocation must carry the max_run_duration description"))

	// The poststop task must NOT have been started.
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		poststopState, ok := alloc.TaskStates["poststop"]
		if !ok {
			return fmt.Errorf("poststop task state must be present in the allocation")
		}
		if poststopState.State != structs.TaskStateDead {
			return fmt.Errorf("poststop task must be marked 'dead': %+v",
				poststopState)
		}
		if !poststopState.StartedAt.IsZero() {
			return fmt.Errorf(
				"poststop task StartedAt must be zero — it was never started: %+v",
				poststopState)
		}
		return nil
	}),
		wait.Timeout(2*time.Second),
		wait.Gap(10*time.Millisecond),
	))

}

// testSysbatchJobTimesOut verifies that max_run_duration also works for
// sysbatch jobs.
func testSysbatchJobTimesOut(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/sysbatch_timeout_basic.hcl",
		jobs3.WaitComplete("group"),
		jobs3.Timeout(testTimeout),
	)
	t.Cleanup(cleanup)

	allocs := job.Allocs()
	must.Positive(t, len(allocs),
		must.Sprint("expected at least one allocation for the sysbatch job"))

	for _, alloc := range allocs {
		must.Eq(t, nomadapi.AllocClientStatusComplete, alloc.ClientStatus,
			must.Sprintf("sysbatch alloc %s must be 'complete' after timeout", alloc.ID[:8]))
		must.Eq(t, structs.AllocTimeoutReasonMaxRunDuration, alloc.ClientDescription,
			must.Sprintf("sysbatch alloc %s must carry the max_run_duration description", alloc.ID[:8]))
	}
}
