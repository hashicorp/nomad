// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

// Stats is used to query the state of the blocked eval tracker. This is only
// ever called in tests.
func (b *BlockedEvals) Stats() *BlockedStats {
	return b.stats.Copy()
}

func testBlockedEvals(t *testing.T) (*BlockedEvals, *EvalBroker) {
	broker := testBroker(t, 0)
	broker.SetEnabled(true)
	blocked := NewBlockedEvals(broker, testlog.HCLogger(t))
	blocked.SetEnabled(true)
	return blocked, broker
}

func TestBlockedEvals_Block_Disabled(t *testing.T) {
	ci.Parallel(t)
	blocked, _ := testBlockedEvals(t)
	blocked.SetEnabled(false)

	// Create an escaped eval and add it to the blocked tracker.
	e := mock.BlockedEval()
	e.EscapedComputedClass = true
	<-blocked.Block(e)

	// Verify block did nothing.
	stats := blocked.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 0, stats.BlockedResources.ByJob)
}

func TestBlockedEvals_Block_SameJob(t *testing.T) {
	ci.Parallel(t)
	blocked, _ := testBlockedEvals(t)

	// Create two blocked evals and add them to the blocked tracker.
	e := mock.BlockedEval()
	e2 := mock.BlockedEval()
	e2.JobID = e.JobID
	<-blocked.Block(e)
	<-blocked.Block(e2)

	// Verify block didn't track duplicate.
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)
}

func TestBlockedEvals_Block_Quota(t *testing.T) {
	ci.Parallel(t)

	blocked, _ := testBlockedEvals(t)

	// Create a blocked eval on quota.
	e := mock.BlockedEval()
	e.QuotaLimitReached = "foo"
	<-blocked.Block(e)

	// Verify block did track eval.
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.Eq(t, 1, stats.TotalQuotaLimit)
}

func TestBlockedEvals_Block_PriorUnblocks(t *testing.T) {
	ci.Parallel(t)
	blocked, _ := testBlockedEvals(t)

	// Do unblocks prior to blocking.
	blocked.Unblock("v1:123", 1000)
	blocked.Unblock("v1:123", 1001)

	// Create blocked eval with two classes ineligible.
	e := mock.BlockedEval()
	e.ClassEligibility = map[string]bool{"v1:123": false, "v1:456": false}
	e.SnapshotIndex = 999
	<-blocked.Block(e)

	// Verify block did track eval.
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)
}

func TestBlockedEvals_GetDuplicates(t *testing.T) {
	ci.Parallel(t)
	blocked, _ := testBlockedEvals(t)

	// Create duplicate blocked evals and add them to the blocked tracker.
	e := mock.BlockedEval()
	e.CreateIndex = 100
	e2 := mock.BlockedEval()
	e2.JobID = e.JobID
	e2.CreateIndex = 101
	e3 := mock.BlockedEval()
	e3.JobID = e.JobID
	e3.CreateIndex = 102
	e4 := mock.BlockedEval()
	e4.JobID = e.JobID
	e4.CreateIndex = 100
	<-blocked.Block(e)
	<-blocked.Block(e2)

	// Verify stats such that we are only tracking one.
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	// Get the duplicates.
	out := blocked.GetDuplicates(0)
	must.Len(t, 1, out)
	must.Eq(t, e, out[0])

	// Call block again after a small sleep.
	go func() {
		time.Sleep(500 * time.Millisecond)
		<-blocked.Block(e3)
	}()

	// Get the duplicates.
	out = blocked.GetDuplicates(1 * time.Second)
	must.Len(t, 1, out)
	must.Eq(t, e2, out[0])

	// Verify stats such that we are only tracking one.
	stats = blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	// Add an older evaluation and assert it gets cancelled.
	<-blocked.Block(e4)
	out = blocked.GetDuplicates(0)
	must.Len(t, 1, out)
	must.Eq(t, e4, out[0])

	// Verify stats such that we are only tracking one.
	stats = blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)
}

func TestBlockedEvals_UnblockEscaped(t *testing.T) {
	ci.Parallel(t)

	blocked, broker := testBlockedEvals(t)

	// Create an escaped eval and add it to the blocked tracker.
	e := mock.BlockedEval()
	e.Status = structs.EvalStatusBlocked
	e.EscapedComputedClass = true
	<-blocked.Block(e)

	// Verify block caused the eval to be tracked
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 1, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	<-blocked.Unblock("v1:123", 1000)
	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

func requireBlockedEvalsEnqueued(t *testing.T, blocked *BlockedEvals, broker *EvalBroker, enqueued int) {
	t.Helper()

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			brokerStats := broker.Stats()
			if brokerStats.TotalReady != enqueued {
				return fmt.Errorf("missing enqueued evals: %#v", brokerStats)
			}
			return nil
		}),
		wait.Timeout(500*time.Millisecond),
		wait.Gap(10*time.Millisecond),
	))

	blocked.pruneStats(time.Now().UTC())

	// verfiy stats have been cleared
	stats := blocked.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 0, stats.BlockedResources.ByJob)
}

func TestBlockedEvals_UnblockEligible(t *testing.T) {
	ci.Parallel(t)
	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.BlockedEval()
	e.Status = structs.EvalStatusBlocked
	e.ClassEligibility = map[string]bool{"v1:123": true}
	<-blocked.Block(e)

	// Verify block caused the eval to be tracked
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)

	blocked.Unblock("v1:123", 1000)
	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

func TestBlockedEvals_UnblockIneligible(t *testing.T) {
	ci.Parallel(t)
	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is ineligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.BlockedEval()
	e.ClassEligibility = map[string]bool{"v1:123": false}
	<-blocked.Block(e)

	// Verify block caused the eval to be tracked
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	// Should do nothing
	<-blocked.Unblock("v1:123", 1000)

	// Verify Unblock didn't cause an enqueue
	brokerStats := broker.Stats()
	must.Eq(t, 0, brokerStats.TotalReady, must.Sprintf("eval unblocked: %#v", brokerStats))

	// Prune old and empty metrics.
	blocked.pruneStats(time.Now().UTC())

	stats = blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)
}

func TestBlockedEvals_UnblockUnknown(t *testing.T) {
	ci.Parallel(t)
	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is ineligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.BlockedEval()
	e.ClassEligibility = map[string]bool{"v1:123": true, "v1:456": false}
	<-blocked.Block(e)

	// Verify block caused the eval to be tracked.
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	// Should unblock because the eval hasn't seen this node class.
	<-blocked.Unblock("v1:789", 1000)
	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

func TestBlockedEvals_UnblockEligible_Quota(t *testing.T) {
	ci.Parallel(t)

	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is eligible for a particular quota.
	e := mock.BlockedEval()
	e.QuotaLimitReached = "foo"
	<-blocked.Block(e)

	// Verify block caused the eval to be tracked.
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 1, stats.TotalQuotaLimit)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	<-blocked.UnblockQuota("foo", 1000)
	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

// The quota here is incidental. The eval is blocked due to something else,
// e.g. cpu exhausted, but there happens to also be a quota on the namespace.
func TestBlockedEvals_UnblockEligible_IncidentalQuota(t *testing.T) {
	ci.Parallel(t)
	blocked, broker := testBlockedEvals(t)

	e := mock.BlockedEval()
	e.Status = structs.EvalStatusBlocked
	e.QuotaLimitReached = "" // explicitly not blocked due to quota limit
	<-blocked.Block(e)

	// Verify block caused the eval to be tracked.
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)
	// but not due to quota.
	must.Eq(t, 0, stats.TotalQuotaLimit)

	// When unblocking, the quota name from the alloc is passed in,
	// regardless of the cause of the initial blockage.
	// Since the initial block in this test was due to something else,
	// it should be unblocked without regard to quota.
	<-blocked.UnblockQuota("foo", 1000)
	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

func TestBlockedEvals_UnblockIneligible_Quota(t *testing.T) {
	ci.Parallel(t)
	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is eligible on a specific quota.
	e := mock.BlockedEval()
	e.QuotaLimitReached = "foo"
	<-blocked.Block(e)

	// Verify block caused the eval to be tracked.
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 1, stats.TotalQuotaLimit)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	// Should do nothing and have no evals enqueued
	<-blocked.UnblockQuota("bar", 1000)
	brokerStats := broker.Stats()
	must.Eq(t, 0, brokerStats.TotalReady)

	// Prune old and empty metrics.
	blocked.pruneStats(time.Now().UTC())

	stats = blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.Eq(t, 1, stats.TotalQuotaLimit)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)
}

func TestBlockedEvals_Reblock(t *testing.T) {
	ci.Parallel(t)
	blocked, broker := testBlockedEvals(t)

	// Create an evaluation, Enqueue/Dequeue it to get a token
	e := mock.BlockedEval()
	e.SnapshotIndex = 500
	e.ClassEligibility = map[string]bool{"v1:123": true, "v1:456": false}
	broker.Enqueue(e)

	_, token, err := broker.Dequeue([]string{e.Type}, time.Second)
	must.NoError(t, err)

	// Reblock the evaluation
	<-blocked.Reblock(e, token)

	// Verify block caused the eval to be tracked
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	// Should unblock because the eval
	<-blocked.Unblock("v1:123", 1000)

	brokerStats := broker.Stats()
	must.Eq(t, 0, brokerStats.TotalReady)
	must.Eq(t, 1, brokerStats.TotalUnacked)

	// Ack the evaluation which should cause the reblocked eval to transition
	// to ready
	err = broker.Ack(e.ID, token)
	must.NoError(t, err)

	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

// Test the block case in which the eval should be immediately unblocked since
// it is escaped and old
func TestBlockedEvals_Block_ImmediateUnblock_Escaped(t *testing.T) {
	ci.Parallel(t)
	blocked, broker := testBlockedEvals(t)

	// Do an unblock prior to blocking
	<-blocked.Unblock("v1:123", 1000)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.BlockedEval()
	e.EscapedComputedClass = true
	e.SnapshotIndex = 900
	<-blocked.Block(e)

	// Verify block caused the eval to be immediately unblocked
	stats := blocked.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 0, stats.BlockedResources.ByJob)

	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

// Test the block case in which the eval should be immediately unblocked since
// there is an unblock on an unseen class that occurred while it was in the
// scheduler
func TestBlockedEvals_Block_ImmediateUnblock_UnseenClass_After(t *testing.T) {
	ci.Parallel(t)

	blocked, broker := testBlockedEvals(t)

	// Do an unblock prior to blocking
	<-blocked.Unblock("v1:123", 1000)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.BlockedEval()
	e.EscapedComputedClass = false
	e.SnapshotIndex = 900
	<-blocked.Block(e)

	// Verify block caused the eval to be immediately unblocked
	stats := blocked.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 0, stats.BlockedResources.ByJob)

	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

// Test the block case in which the eval should not immediately unblock since
// there is an unblock on an unseen class that occurred before it was in the
// scheduler
func TestBlockedEvals_Block_ImmediateUnblock_UnseenClass_Before(t *testing.T) {
	ci.Parallel(t)
	blocked, _ := testBlockedEvals(t)

	// Do an unblock prior to blocking
	<-blocked.Unblock("v1:123", 500)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.BlockedEval()
	e.EscapedComputedClass = false
	e.SnapshotIndex = 900
	<-blocked.Block(e)

	// Verify block caused the eval to be immediately unblocked
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)
}

// Test the block case in which the eval should be immediately unblocked since
// it a class it is eligible for has been unblocked
func TestBlockedEvals_Block_ImmediateUnblock_SeenClass(t *testing.T) {
	ci.Parallel(t)
	blocked, broker := testBlockedEvals(t)

	// Do an unblock prior to blocking
	<-blocked.Unblock("v1:123", 1000)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.BlockedEval()
	e.ClassEligibility = map[string]bool{"v1:123": true, "v1:456": false}
	e.SnapshotIndex = 900
	<-blocked.Block(e)

	// Verify block caused the eval to be immediately unblocked
	stats := blocked.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 0, stats.BlockedResources.ByJob)

	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

// Test the block case in which the eval should be immediately unblocked since
// it a quota has changed that it is using
func TestBlockedEvals_Block_ImmediateUnblock_Quota(t *testing.T) {
	ci.Parallel(t)
	blocked, broker := testBlockedEvals(t)

	// Do an unblock prior to blocking
	<-blocked.UnblockQuota("my-quota", 1000)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.BlockedEval()
	e.QuotaLimitReached = "my-quota"
	e.SnapshotIndex = 900
	<-blocked.Block(e)

	// Verify block caused the eval to be immediately unblocked
	stats := blocked.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.Eq(t, 0, stats.TotalQuotaLimit)
	must.MapLen(t, 0, stats.BlockedResources.ByJob)

	requireBlockedEvalsEnqueued(t, blocked, broker, 1)
}

func TestBlockedEvals_UnblockFailed(t *testing.T) {
	ci.Parallel(t)

	blocked, broker := testBlockedEvals(t)

	// Create blocked evals that are due to failures
	e := mock.BlockedEval()
	e.TriggeredBy = structs.EvalTriggerMaxPlans
	e.EscapedComputedClass = true
	<-blocked.Block(e)

	e2 := mock.BlockedEval()
	e2.Status = structs.EvalStatusBlocked
	e2.TriggeredBy = structs.EvalTriggerMaxPlans
	e2.ClassEligibility = map[string]bool{"v1:123": true, "v1:456": false}
	<-blocked.Block(e2)

	e3 := mock.BlockedEval()
	e3.TriggeredBy = structs.EvalTriggerMaxPlans
	e3.QuotaLimitReached = "foo"
	<-blocked.Block(e3)

	// Trigger an unblock fail
	blocked.UnblockFailed()

	// Prune old and empty metrics.
	blocked.pruneStats(time.Now().UTC())

	// Verify UnblockFailed caused the eval to be immediately unblocked
	stats := blocked.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.Eq(t, 0, stats.TotalQuotaLimit)
	must.MapLen(t, 0, stats.BlockedResources.ByJob)

	requireBlockedEvalsEnqueued(t, blocked, broker, 3)

	// Reblock an eval for the same job and check that it gets tracked.
	<-blocked.Block(e)
	stats = blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 1, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)
}

func TestBlockedEvals_Untrack(t *testing.T) {
	ci.Parallel(t)
	blocked, _ := testBlockedEvals(t)

	// Create blocked eval and add to the blocked tracker.
	e := mock.BlockedEval()
	e.ClassEligibility = map[string]bool{"v1:123": false, "v1:456": false}
	e.SnapshotIndex = 1000
	<-blocked.Block(e)

	// Verify block did track
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	// Untrack and verify
	<-blocked.Untrack(e.JobID, e.Namespace)
	blocked.pruneStats(time.Now().UTC())

	stats = blocked.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.MapLen(t, 0, stats.BlockedResources.ByJob)
}

func TestBlockedEvals_Untrack_Quota(t *testing.T) {
	ci.Parallel(t)
	blocked, _ := testBlockedEvals(t)

	// Create a blocked eval and add it to the blocked tracker.
	e := mock.BlockedEval()
	e.QuotaLimitReached = "foo"
	e.SnapshotIndex = 1000
	<-blocked.Block(e)

	// Verify block did track
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.Eq(t, 1, stats.TotalQuotaLimit)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	// Untrack and verify
	<-blocked.Untrack(e.JobID, e.Namespace)
	stats = blocked.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.Eq(t, 0, stats.TotalQuotaLimit)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	now := time.Now().UTC()
	blocked.pruneStats(now)
	stats = blocked.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.Eq(t, 0, stats.TotalQuotaLimit)
	must.MapLen(t, 0, stats.BlockedResources.ByJob)
}

func TestBlockedEvals_UnblockNode(t *testing.T) {
	ci.Parallel(t)

	blocked, broker := testBlockedEvals(t)

	// Create a blocked evals and add it to the blocked tracker.
	e := mock.BlockedEval()
	e.Type = structs.JobTypeSystem
	e.NodeID = "foo"
	e.SnapshotIndex = 999
	<-blocked.Block(e)

	// Verify block did track
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	<-blocked.UnblockNode("foo")
	requireBlockedEvalsEnqueued(t, blocked, broker, 1)

	blocked.pruneStats(time.Now().UTC())
	stats = blocked.Stats()
	must.MapEmpty(t, blocked.system.byNode)
	must.Eq(t, 0, stats.TotalBlocked)
	must.MapLen(t, 0, stats.BlockedResources.ByJob)
}

func TestBlockedEvals_SystemUntrack(t *testing.T) {
	ci.Parallel(t)
	blocked, _ := testBlockedEvals(t)

	// Create a blocked evals and add it to the blocked tracker.
	e := mock.Eval()
	e.Type = structs.JobTypeSystem
	e.NodeID = "foo"
	<-blocked.Block(e)

	// Verify block did track
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.Eq(t, 0, stats.TotalQuotaLimit)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	// Untrack and verify
	<-blocked.Untrack(e.JobID, e.Namespace)
	blocked.pruneStats(time.Now().UTC())

	stats = blocked.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.Eq(t, 0, stats.TotalQuotaLimit)
	must.MapLen(t, 0, stats.BlockedResources.ByJob)
}

func TestBlockedEvals_SystemDisableFlush(t *testing.T) {
	ci.Parallel(t)
	blocked, _ := testBlockedEvals(t)

	// Create a blocked evals and add it to the blocked tracker.
	e := mock.Eval()
	e.Type = structs.JobTypeSystem
	e.NodeID = "foo"
	<-blocked.Block(e)

	// Verify block did track
	stats := blocked.Stats()
	must.Eq(t, 1, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.Eq(t, 0, stats.TotalQuotaLimit)
	must.MapLen(t, 1, stats.BlockedResources.ByJob)

	// Disable empties
	blocked.SetEnabled(false)
	stats = blocked.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
	must.Eq(t, 0, stats.TotalQuotaLimit)
	must.MapLen(t, 0, stats.BlockedResources.ByJob)
	must.MapEmpty(t, blocked.system.evals)
	must.MapEmpty(t, blocked.system.byJob)
	must.MapEmpty(t, blocked.system.byNode)
}

// TestBlockedEvals_UnblockBackpressure verifies that calling Unblock methods on
// BlockedEvals doesn't result in backpressure that could impact the FSM.
func TestBlockedEvals_UnblockBackpressure(t *testing.T) {
	pctx, cancel := context.WithCancel(context.TODO())
	t.Cleanup(cancel)

	broker, err := NewEvalBroker(pctx, time.Second, 1, 1, 1)
	must.NoError(t, err)
	broker.SetEnabled(true)
	blockedEvals := NewBlockedEvals(broker, testlog.HCLogger(t))
	blockedEvals.SetEnabled(true)

	for range 10000 {
		eval := mock.Eval()
		eval.QueuedAllocations = map[string]int{"web": 1}
		eval.ClassEligibility = map[string]bool{"foo": false}
		eval.EscapedComputedClass = true
		eval.Status = structs.EvalStatusBlocked

		<-blockedEvals.Block(eval)
	}
	stats := blockedEvals.Stats()
	must.Eq(t, 10000, stats.TotalBlocked)
	must.Eq(t, 10000, stats.TotalEscaped)

	stopCtx, stopCancel := context.WithCancel(pctx)
	t.Cleanup(stopCancel)

	var wg sync.WaitGroup
	errors := make(chan error, 144)

	// this loop continuously calls unblock to trigger any contention we might
	// have; these calls should return quickly but be no-ops in watchCapacity
	// once we've unblocked all 10000 blocked evals
	//
	// Note that this test isn't particularly useful in CI where we have low
	// GOMAXPROCS. And with the fix it's intended to exercise, these Unblock
	// calls return too quickly for even highly concurrent continuous Reblock
	// loops to cause meaningful contention.
	for i := range 144 {
		wg.Go(func() {
			index := uint64(1000 * i)
			for {
				ctx, cancel := context.WithTimeout(stopCtx, 100*time.Millisecond)
				defer cancel()
				errCh := make(chan error)
				index++
				go func() {
					blockedEvals.Unblock("foo", index)
					close(errCh)
				}()

				select {
				case <-ctx.Done():
					if ctx.Err() == context.DeadlineExceeded {
						errors <- fmt.Errorf("Unblock took more than 100ms to complete")
					}
					return
				case <-errCh:
					cancel()
					continue
				}
			}

		})
	}

	// wait until all 10000 evals are unblocked or 5 seconds pass, whichever is
	// longer, and collect any errors from goroutines that would indicate
	// they've blocked for longer than expected
	timeout := time.After(time.Second * 5)
	poll := time.NewTicker(10 * time.Millisecond)
	errCount := 0
	var lastErr error

DONE:
	for {
		select {
		case <-timeout:
			break DONE
		case <-poll.C:
			stats = blockedEvals.Stats()
			if stats.TotalBlocked == 0 && stats.TotalEscaped == 0 {
				break DONE
			}
		case err := <-errors:
			if err != nil {
				lastErr = err
				errCount++
			}
		}
	}

	must.NoError(t, lastErr, must.Sprintf("got %d errors", errCount))
	stats = blockedEvals.Stats()
	must.Eq(t, 0, stats.TotalBlocked)
	must.Eq(t, 0, stats.TotalEscaped)
}
