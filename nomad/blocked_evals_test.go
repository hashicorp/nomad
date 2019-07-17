package nomad

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func testBlockedEvals(t *testing.T) (*BlockedEvals, *EvalBroker) {
	broker := testBroker(t, 0)
	broker.SetEnabled(true)
	blocked := NewBlockedEvals(broker, testlog.HCLogger(t))
	blocked.SetEnabled(true)
	return blocked, broker
}

func TestBlockedEvals_Block_Disabled(t *testing.T) {
	t.Parallel()
	blocked, _ := testBlockedEvals(t)
	blocked.SetEnabled(false)

	// Create an escaped eval and add it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.EscapedComputedClass = true
	blocked.Block(e)

	// Verify block did nothing
	bStats := blocked.Stats()
	if bStats.TotalBlocked != 0 || bStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", bStats)
	}
}

func TestBlockedEvals_Block_SameJob(t *testing.T) {
	t.Parallel()
	blocked, _ := testBlockedEvals(t)

	// Create two blocked evals and add them to the blocked tracker.
	e := mock.Eval()
	e2 := mock.Eval()
	e2.JobID = e.JobID
	blocked.Block(e)
	blocked.Block(e2)

	// Verify block did track both
	bStats := blocked.Stats()
	if bStats.TotalBlocked != 1 || bStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", bStats)
	}
}

func TestBlockedEvals_Block_Quota(t *testing.T) {
	t.Parallel()
	blocked, _ := testBlockedEvals(t)

	// Create a blocked evals on quota
	e := mock.Eval()
	e.QuotaLimitReached = "foo"
	blocked.Block(e)

	// Verify block did track both
	bs := blocked.Stats()
	if bs.TotalBlocked != 1 || bs.TotalEscaped != 0 || bs.TotalQuotaLimit != 1 {
		t.Fatalf("bad: %#v", bs)
	}
}

func TestBlockedEvals_Block_PriorUnblocks(t *testing.T) {
	t.Parallel()
	blocked, _ := testBlockedEvals(t)

	// Do unblocks prior to blocking
	blocked.Unblock("v1:123", 1000)
	blocked.Unblock("v1:123", 1001)

	// Create two blocked evals and add them to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.ClassEligibility = map[string]bool{"v1:123": false, "v1:456": false}
	e.SnapshotIndex = 999
	blocked.Block(e)

	// Verify block did track both
	bStats := blocked.Stats()
	if bStats.TotalBlocked != 1 || bStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", bStats)
	}
}

func TestBlockedEvals_GetDuplicates(t *testing.T) {
	t.Parallel()
	blocked, _ := testBlockedEvals(t)

	// Create duplicate blocked evals and add them to the blocked tracker.
	e := mock.Eval()
	e.CreateIndex = 100
	e2 := mock.Eval()
	e2.JobID = e.JobID
	e2.CreateIndex = 101
	e3 := mock.Eval()
	e3.JobID = e.JobID
	e3.CreateIndex = 102
	e4 := mock.Eval()
	e4.JobID = e.JobID
	e4.CreateIndex = 100
	blocked.Block(e)
	blocked.Block(e2)

	// Verify stats such that we are only tracking one
	bStats := blocked.Stats()
	if bStats.TotalBlocked != 1 || bStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", bStats)
	}

	// Get the duplicates.
	out := blocked.GetDuplicates(0)
	if len(out) != 1 || !reflect.DeepEqual(out[0], e) {
		t.Fatalf("bad: %#v %#v", out, e)
	}

	// Call block again after a small sleep.
	go func() {
		time.Sleep(500 * time.Millisecond)
		blocked.Block(e3)
	}()

	// Get the duplicates.
	out = blocked.GetDuplicates(1 * time.Second)
	if len(out) != 1 || !reflect.DeepEqual(out[0], e2) {
		t.Fatalf("bad: %#v %#v", out, e2)
	}

	// Verify stats such that we are only tracking one
	bStats = blocked.Stats()
	if bStats.TotalBlocked != 1 || bStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", bStats)
	}

	// Add an older evaluation and assert it gets cancelled
	blocked.Block(e4)
	out = blocked.GetDuplicates(0)
	if len(out) != 1 || !reflect.DeepEqual(out[0], e4) {
		t.Fatalf("bad: %#v %#v", out, e4)
	}

	// Verify stats such that we are only tracking one
	bStats = blocked.Stats()
	if bStats.TotalBlocked != 1 || bStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", bStats)
	}
}

func TestBlockedEvals_UnblockEscaped(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	// Create an escaped eval and add it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.EscapedComputedClass = true
	blocked.Block(e)

	// Verify block caused the eval to be tracked
	bStats := blocked.Stats()
	if bStats.TotalBlocked != 1 || bStats.TotalEscaped != 1 {
		t.Fatalf("bad: %#v", bStats)
	}

	blocked.Unblock("v1:123", 1000)
	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

func requireBlockedEvalsEnqueued(t *testing.T, blocked *BlockedEvals, broker *EvalBroker, enqueued int) {
	testutil.WaitForResult(func() (bool, error) {
		// Verify Unblock caused an enqueue
		brokerStats := broker.Stats()
		if brokerStats.TotalReady != enqueued {
			return false, fmt.Errorf("missing enqueued evals: %#v", brokerStats)
		}

		// Verify Unblock updates the stats
		bStats := blocked.Stats()
		if bStats.TotalBlocked != 0 || bStats.TotalEscaped != 0 {
			return false, fmt.Errorf("evals still blocked: %#v", bStats)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestBlockedEvals_UnblockEligible(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.ClassEligibility = map[string]bool{"v1:123": true}
	blocked.Block(e)

	// Verify block caused the eval to be tracked
	blockedStats := blocked.Stats()
	if blockedStats.TotalBlocked != 1 {
		t.Fatalf("bad: %#v", blockedStats)
	}

	blocked.Unblock("v1:123", 1000)
	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

func TestBlockedEvals_UnblockIneligible(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is ineligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.ClassEligibility = map[string]bool{"v1:123": false}
	blocked.Block(e)

	// Verify block caused the eval to be tracked
	blockedStats := blocked.Stats()
	if blockedStats.TotalBlocked != 1 && blockedStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}

	// Should do nothing
	blocked.Unblock("v1:123", 1000)

	testutil.WaitForResult(func() (bool, error) {
		// Verify Unblock didn't cause an enqueue
		brokerStats := broker.Stats()
		if brokerStats.TotalReady != 0 {
			return false, fmt.Errorf("bad: %#v", brokerStats)
		}

		bStats := blocked.Stats()
		if bStats.TotalBlocked != 1 || bStats.TotalEscaped != 0 {
			return false, fmt.Errorf("bad: %#v", bStats)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestBlockedEvals_UnblockUnknown(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is ineligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.ClassEligibility = map[string]bool{"v1:123": true, "v1:456": false}
	blocked.Block(e)

	// Verify block caused the eval to be tracked
	blockedStats := blocked.Stats()
	if blockedStats.TotalBlocked != 1 && blockedStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}

	// Should unblock because the eval hasn't seen this node class.
	blocked.Unblock("v1:789", 1000)
	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

func TestBlockedEvals_UnblockEligible_Quota(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is eligible for a particular quota
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.QuotaLimitReached = "foo"
	blocked.Block(e)

	// Verify block caused the eval to be tracked
	bs := blocked.Stats()
	if bs.TotalBlocked != 1 || bs.TotalQuotaLimit != 1 {
		t.Fatalf("bad: %#v", bs)
	}

	blocked.UnblockQuota("foo", 1000)
	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

func TestBlockedEvals_UnblockIneligible_Quota(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is eligible on a specific quota
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.QuotaLimitReached = "foo"
	blocked.Block(e)

	// Verify block caused the eval to be tracked
	bs := blocked.Stats()
	if bs.TotalBlocked != 1 || bs.TotalQuotaLimit != 1 {
		t.Fatalf("bad: %#v", bs)
	}

	// Should do nothing
	blocked.UnblockQuota("bar", 1000)

	testutil.WaitForResult(func() (bool, error) {
		// Verify Unblock didn't cause an enqueue
		brokerStats := broker.Stats()
		if brokerStats.TotalReady != 0 {
			return false, fmt.Errorf("bad: %#v", brokerStats)
		}

		bs := blocked.Stats()
		if bs.TotalBlocked != 1 || bs.TotalEscaped != 0 || bs.TotalQuotaLimit != 1 {
			return false, fmt.Errorf("bad: %#v", bs)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestBlockedEvals_Reblock(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	// Create an evaluation, Enqueue/Dequeue it to get a token
	e := mock.Eval()
	e.SnapshotIndex = 500
	e.Status = structs.EvalStatusBlocked
	e.ClassEligibility = map[string]bool{"v1:123": true, "v1:456": false}
	broker.Enqueue(e)

	_, token, err := broker.Dequeue([]string{e.Type}, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Reblock the evaluation
	blocked.Reblock(e, token)

	// Verify block caused the eval to be tracked
	blockedStats := blocked.Stats()
	if blockedStats.TotalBlocked != 1 && blockedStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}

	// Should unblock because the eval
	blocked.Unblock("v1:123", 1000)

	brokerStats := broker.Stats()
	if brokerStats.TotalReady != 0 && brokerStats.TotalUnacked != 1 {
		t.Fatalf("bad: %#v", brokerStats)
	}

	// Ack the evaluation which should cause the reblocked eval to transition
	// to ready
	if err := broker.Ack(e.ID, token); err != nil {
		t.Fatalf("err: %v", err)
	}

	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

// Test the block case in which the eval should be immediately unblocked since
// it is escaped and old
func TestBlockedEvals_Block_ImmediateUnblock_Escaped(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	// Do an unblock prior to blocking
	blocked.Unblock("v1:123", 1000)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.EscapedComputedClass = true
	e.SnapshotIndex = 900
	blocked.Block(e)

	// Verify block caused the eval to be immediately unblocked
	blockedStats := blocked.Stats()
	if blockedStats.TotalBlocked != 0 && blockedStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}

	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

// Test the block case in which the eval should be immediately unblocked since
// there is an unblock on an unseen class that occurred while it was in the
// scheduler
func TestBlockedEvals_Block_ImmediateUnblock_UnseenClass_After(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	// Do an unblock prior to blocking
	blocked.Unblock("v1:123", 1000)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.EscapedComputedClass = false
	e.SnapshotIndex = 900
	blocked.Block(e)

	// Verify block caused the eval to be immediately unblocked
	blockedStats := blocked.Stats()
	if blockedStats.TotalBlocked != 0 && blockedStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}

	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

// Test the block case in which the eval should not immediately unblock since
// there is an unblock on an unseen class that occurred before it was in the
// scheduler
func TestBlockedEvals_Block_ImmediateUnblock_UnseenClass_Before(t *testing.T) {
	t.Parallel()
	blocked, _ := testBlockedEvals(t)

	// Do an unblock prior to blocking
	blocked.Unblock("v1:123", 500)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.EscapedComputedClass = false
	e.SnapshotIndex = 900
	blocked.Block(e)

	// Verify block caused the eval to be immediately unblocked
	blockedStats := blocked.Stats()
	if blockedStats.TotalBlocked != 1 && blockedStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}
}

// Test the block case in which the eval should be immediately unblocked since
// it a class it is eligible for has been unblocked
func TestBlockedEvals_Block_ImmediateUnblock_SeenClass(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	// Do an unblock prior to blocking
	blocked.Unblock("v1:123", 1000)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.ClassEligibility = map[string]bool{"v1:123": true, "v1:456": false}
	e.SnapshotIndex = 900
	blocked.Block(e)

	// Verify block caused the eval to be immediately unblocked
	blockedStats := blocked.Stats()
	if blockedStats.TotalBlocked != 0 && blockedStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}

	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

// Test the block case in which the eval should be immediately unblocked since
// it a quota has changed that it is using
func TestBlockedEvals_Block_ImmediateUnblock_Quota(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	// Do an unblock prior to blocking
	blocked.UnblockQuota("my-quota", 1000)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.QuotaLimitReached = "my-quota"
	e.SnapshotIndex = 900
	blocked.Block(e)

	// Verify block caused the eval to be immediately unblocked
	bs := blocked.Stats()
	if bs.TotalBlocked != 0 && bs.TotalEscaped != 0 && bs.TotalQuotaLimit != 0 {
		t.Fatalf("bad: %#v", bs)
	}

	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

func TestBlockedEvals_UnblockFailed(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	// Create blocked evals that are due to failures
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.TriggeredBy = structs.EvalTriggerMaxPlans
	e.EscapedComputedClass = true
	blocked.Block(e)

	e2 := mock.Eval()
	e2.Status = structs.EvalStatusBlocked
	e2.TriggeredBy = structs.EvalTriggerMaxPlans
	e2.ClassEligibility = map[string]bool{"v1:123": true, "v1:456": false}
	blocked.Block(e2)

	e3 := mock.Eval()
	e3.Status = structs.EvalStatusBlocked
	e3.TriggeredBy = structs.EvalTriggerMaxPlans
	e3.QuotaLimitReached = "foo"
	blocked.Block(e3)

	// Trigger an unblock fail
	blocked.UnblockFailed()

	// Verify UnblockFailed caused the eval to be immediately unblocked
	bs := blocked.Stats()
	if bs.TotalBlocked != 0 || bs.TotalEscaped != 0 || bs.TotalQuotaLimit != 0 {
		t.Fatalf("bad: %#v", bs)
	}

	requireBlockedEvalsEnqueued(t, blocked, broker, 3)

	// Reblock an eval for the same job and check that it gets tracked.
	blocked.Block(e)
	bs = blocked.Stats()
	if bs.TotalBlocked != 1 || bs.TotalEscaped != 1 {
		t.Fatalf("bad: %#v", bs)
	}
}

func TestBlockedEvals_Untrack(t *testing.T) {
	t.Parallel()
	blocked, _ := testBlockedEvals(t)

	// Create two blocked evals and add them to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.ClassEligibility = map[string]bool{"v1:123": false, "v1:456": false}
	e.SnapshotIndex = 1000
	blocked.Block(e)

	// Verify block did track
	bStats := blocked.Stats()
	if bStats.TotalBlocked != 1 || bStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", bStats)
	}

	// Untrack and verify
	blocked.Untrack(e.JobID, e.Namespace)
	bStats = blocked.Stats()
	if bStats.TotalBlocked != 0 || bStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", bStats)
	}
}

func TestBlockedEvals_Untrack_Quota(t *testing.T) {
	t.Parallel()
	blocked, _ := testBlockedEvals(t)

	// Create a blocked evals and add it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.QuotaLimitReached = "foo"
	e.SnapshotIndex = 1000
	blocked.Block(e)

	// Verify block did track
	bs := blocked.Stats()
	if bs.TotalBlocked != 1 || bs.TotalEscaped != 0 || bs.TotalQuotaLimit != 1 {
		t.Fatalf("bad: %#v", bs)
	}

	// Untrack and verify
	blocked.Untrack(e.JobID, e.Namespace)
	bs = blocked.Stats()
	if bs.TotalBlocked != 0 || bs.TotalEscaped != 0 || bs.TotalQuotaLimit != 0 {
		t.Fatalf("bad: %#v", bs)
	}
}

func TestBlockedEvals_UnblockNode(t *testing.T) {
	t.Parallel()
	blocked, broker := testBlockedEvals(t)

	require.NotNil(t, broker)

	// Create a blocked evals and add it to the blocked tracker.
	e := mock.Eval()
	e.Type = structs.JobTypeSystem
	e.NodeID = "foo"
	e.SnapshotIndex = 999
	blocked.Block(e)

	// Verify block did track
	bs := blocked.Stats()
	require.Equal(t, 1, bs.TotalBlocked)

	blocked.UnblockNode("foo", 1000)
	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
	bs = blocked.Stats()
	require.Empty(t, blocked.system.byNode)
	require.Equal(t, 0, bs.TotalBlocked)
}

func TestBlockedEvals_SystemUntrack(t *testing.T) {
	t.Parallel()
	blocked, _ := testBlockedEvals(t)

	// Create a blocked evals and add it to the blocked tracker.
	e := mock.Eval()
	e.Type = structs.JobTypeSystem
	e.NodeID = "foo"
	blocked.Block(e)

	// Verify block did track
	bs := blocked.Stats()
	require.Equal(t, 1, bs.TotalBlocked)
	require.Equal(t, 0, bs.TotalEscaped)
	require.Equal(t, 0, bs.TotalQuotaLimit)

	// Untrack and verify
	blocked.Untrack(e.JobID, e.Namespace)
	bs = blocked.Stats()
	require.Equal(t, 0, bs.TotalBlocked)
	require.Equal(t, 0, bs.TotalEscaped)
	require.Equal(t, 0, bs.TotalQuotaLimit)
}

func TestBlockedEvals_SystemDisableFlush(t *testing.T) {
	t.Parallel()
	blocked, _ := testBlockedEvals(t)

	// Create a blocked evals and add it to the blocked tracker.
	e := mock.Eval()
	e.Type = structs.JobTypeSystem
	e.NodeID = "foo"
	blocked.Block(e)

	// Verify block did track
	bs := blocked.Stats()
	require.Equal(t, 1, bs.TotalBlocked)
	require.Equal(t, 0, bs.TotalEscaped)
	require.Equal(t, 0, bs.TotalQuotaLimit)

	// Disable empties
	blocked.SetEnabled(false)
	bs = blocked.Stats()
	require.Equal(t, 0, bs.TotalBlocked)
	require.Equal(t, 0, bs.TotalEscaped)
	require.Equal(t, 0, bs.TotalQuotaLimit)
	require.Empty(t, blocked.system.evals)
	require.Empty(t, blocked.system.byJob)
	require.Empty(t, blocked.system.byNode)
}
