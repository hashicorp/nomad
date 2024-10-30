// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package drainer

import (
	"fmt"
	"testing"
	"time"

	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// TestNodeDrainWatcher_AddNodes tests that new nodes are added to the node
// watcher and deadline notifier, but only if they have a drain spec.
func TestNodeDrainWatcher_AddNodes(t *testing.T) {
	ci.Parallel(t)
	_, store, tracker := testNodeDrainWatcher(t)

	// Create two nodes, one draining and one not draining
	n1, n2 := mock.Node(), mock.Node()
	n2.DrainStrategy = &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: time.Hour,
		},
		ForceDeadline: time.Now().Add(time.Hour),
	}

	// Create a job with a running alloc on each node
	job := mock.Job()
	jobID := structs.NamespacedID{Namespace: job.Namespace, ID: job.ID}
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 101, nil, job))

	alloc1 := mock.Alloc()
	alloc1.JobID = job.ID
	alloc1.Job = job
	alloc1.TaskGroup = job.TaskGroups[0].Name
	alloc1.NodeID = n1.ID
	alloc1.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: pointer.Of(true)}
	alloc2 := alloc1.Copy()
	alloc2.ID = uuid.Generate()
	alloc2.NodeID = n2.ID

	must.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, 102, []*structs.Allocation{alloc1, alloc2}))
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 103, n1))
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 104, n2))

	// Only 1 node is draining, and the other should not be tracked
	assertTrackerSettled(t, tracker, []string{n2.ID})

	// Notifications should fire to the job watcher and deadline notifier
	must.MapContainsKey(t, tracker.jobWatcher.(*MockJobWatcher).jobs, jobID)
	must.MapContainsKey(t, tracker.deadlineNotifier.(*MockDeadlineNotifier).nodes, n2.ID)
}

// TestNodeDrainWatcher_Remove tests that when a node should no longer be
// tracked that we stop tracking it in the node watcher and deadline notifier.
func TestNodeDrainWatcher_Remove(t *testing.T) {
	ci.Parallel(t)
	_, store, tracker := testNodeDrainWatcher(t)

	t.Run("stop drain", func(t *testing.T) {
		n, _ := testNodeDrainWatcherSetup(t, store, tracker)

		index, _ := store.LatestIndex()
		must.NoError(t, store.UpdateNodeDrain(
			structs.MsgTypeTestSetup, index+1, n.ID, nil, false, 0, nil, nil, ""))

		// Node with stopped drain should no longer be tracked
		assertTrackerSettled(t, tracker, []string{})
		must.MapEmpty(t, tracker.deadlineNotifier.(*MockDeadlineNotifier).nodes)
	})

	t.Run("delete node", func(t *testing.T) {
		n, _ := testNodeDrainWatcherSetup(t, store, tracker)
		index, _ := store.LatestIndex()
		index++
		must.NoError(t, store.DeleteNode(structs.MsgTypeTestSetup, index, []string{n.ID}))

		// Node with stopped drain should no longer be tracked
		assertTrackerSettled(t, tracker, []string{})
		must.MapEmpty(t, tracker.deadlineNotifier.(*MockDeadlineNotifier).nodes)
	})
}

// TestNodeDrainWatcher_NoRemove tests that when the node status changes to
// down/disconnected that we don't remove it from the node watcher or deadline
// notifier
func TestNodeDrainWatcher_NoRemove(t *testing.T) {
	ci.Parallel(t)
	_, store, tracker := testNodeDrainWatcher(t)
	n, _ := testNodeDrainWatcherSetup(t, store, tracker)

	index, _ := store.LatestIndex()
	n = n.Copy()
	n.Status = structs.NodeStatusDisconnected
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, index+1, n))

	assertTrackerSettled(t, tracker, []string{n.ID})
	must.MapContainsKey(t, tracker.deadlineNotifier.(*MockDeadlineNotifier).nodes, n.ID)

	index, _ = store.LatestIndex()
	n = n.Copy()
	n.Status = structs.NodeStatusDown
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, index+1, n))

	assertTrackerSettled(t, tracker, []string{n.ID})
	must.MapContainsKey(t, tracker.deadlineNotifier.(*MockDeadlineNotifier).nodes, n.ID)
}

// TestNodeDrainWatcher_Update_Spec tests drain spec updates emit events to the
// node watcher and deadline notifier.
func TestNodeDrainWatcher_Update_Spec(t *testing.T) {
	ci.Parallel(t)
	_, store, tracker := testNodeDrainWatcher(t)
	n, _ := testNodeDrainWatcherSetup(t, store, tracker)

	// Update the spec to extend the deadline
	strategy := n.DrainStrategy.Copy()
	strategy.DrainSpec.Deadline += time.Hour
	index, _ := store.LatestIndex()
	must.NoError(t, store.UpdateNodeDrain(
		structs.MsgTypeTestSetup, index+1, n.ID, strategy, false, time.Now().Unix(),
		&structs.NodeEvent{}, map[string]string{}, "",
	))

	// We should see a new event
	assertTrackerSettled(t, tracker, []string{n.ID})

	// Update the spec to have an infinite deadline
	strategy = strategy.Copy()
	strategy.DrainSpec.Deadline = 0

	index, _ = store.LatestIndex()
	must.NoError(t, store.UpdateNodeDrain(
		structs.MsgTypeTestSetup, index+1, n.ID, strategy, false, time.Now().Unix(),
		&structs.NodeEvent{}, map[string]string{}, "",
	))

	// We should see a new event and the node should still be tracked but no
	// longer in the deadline notifier
	assertTrackerSettled(t, tracker, []string{n.ID})
	must.MapEmpty(t, tracker.deadlineNotifier.(*MockDeadlineNotifier).nodes)
}

// TestNodeDrainWatcher_Update_IsDone tests that a node drain without allocs
// immediately gets unmarked as draining, and that we unset drain if an operator
// drains a node with nothing on it.
func TestNodeDrainWatcher_Update_IsDone(t *testing.T) {
	ci.Parallel(t)
	_, store, tracker := testNodeDrainWatcher(t)

	// Create a draining node
	n := mock.Node()
	strategy := &structs.DrainStrategy{
		DrainSpec:     structs.DrainSpec{Deadline: time.Hour},
		ForceDeadline: time.Now().Add(time.Hour),
	}
	n.DrainStrategy = strategy
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 100, n))

	// There are no jobs on this node so the drain should immediately
	// complete. we should no longer be tracking the node and its drain strategy
	// should be cleared
	assertTrackerSettled(t, tracker, []string{})
	must.MapEmpty(t, tracker.jobWatcher.(*MockJobWatcher).jobs)
	must.MapEmpty(t, tracker.deadlineNotifier.(*MockDeadlineNotifier).nodes)
	n, _ = store.NodeByID(nil, n.ID)
	must.Nil(t, n.DrainStrategy)
}

// TestNodeDrainWatcher_Update_DrainComplete tests that allocation updates that
// complete the drain emits events to the node watcher and deadline notifier.
func TestNodeDrainWatcher_Update_DrainComplete(t *testing.T) {
	ci.Parallel(t)
	_, store, tracker := testNodeDrainWatcher(t)
	n, _ := testNodeDrainWatcherSetup(t, store, tracker)

	// Simulate event: an alloc is terminal so DrainingJobWatcher.Migrated
	// channel updates NodeDrainer, which updates Raft
	_, err := tracker.raft.NodesDrainComplete([]string{n.ID},
		structs.NewNodeEvent().
			SetSubsystem(structs.NodeEventSubsystemDrain).
			SetMessage(NodeDrainEventComplete))
	must.NoError(t, err)

	assertTrackerSettled(t, tracker, []string{})

	n, _ = store.NodeByID(nil, n.ID)
	must.Nil(t, n.DrainStrategy)
	must.MapEmpty(t, tracker.deadlineNotifier.(*MockDeadlineNotifier).nodes)
}

func testNodeDrainWatcherSetup(
	t *testing.T, store *state.StateStore, tracker *NodeDrainer) (
	*structs.Node, structs.NamespacedID) {

	t.Helper()
	index, _ := store.LatestIndex()

	// Create a job that will have an alloc on our node
	job := mock.Job()
	jobID := structs.NamespacedID{Namespace: job.Namespace, ID: job.ID}
	index++
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job))

	// Create draining nodes, each with its own alloc for the job running on that node
	node := mock.Node()
	node.DrainStrategy = &structs.DrainStrategy{
		DrainSpec:     structs.DrainSpec{Deadline: time.Hour},
		ForceDeadline: time.Now().Add(time.Hour),
	}

	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.Job = job
	alloc.TaskGroup = job.TaskGroups[0].Name
	alloc.NodeID = node.ID
	alloc.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: pointer.Of(true)}
	index++
	must.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc}))

	index++
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, index, node))

	// Node should be tracked and notifications should fire to the job watcher
	// and deadline notifier
	assertTrackerSettled(t, tracker, []string{node.ID})
	must.MapContainsKey(t, tracker.jobWatcher.(*MockJobWatcher).jobs, jobID)
	must.MapContainsKeys(t,
		tracker.deadlineNotifier.(*MockDeadlineNotifier).nodes, []string{node.ID})

	return node, jobID
}

func assertTrackerSettled(t *testing.T, tracker *NodeDrainer, nodeIDs []string) {
	t.Helper()

	must.Wait(t, wait.InitialSuccess(
		wait.Timeout(100*time.Millisecond),
		wait.Gap(time.Millisecond),
		wait.TestFunc(func() (bool, error) {
			if len(tracker.TrackedNodes()) != len(nodeIDs) {
				return false, fmt.Errorf(
					"expected nodes %v to become marked draining, got %d",
					nodeIDs, len(tracker.TrackedNodes()))
			}
			return true, nil
		}),
	))

	must.Wait(t, wait.ContinualSuccess(
		wait.Timeout(100*time.Millisecond),
		wait.Gap(10*time.Millisecond),
		wait.TestFunc(func() (bool, error) {
			if len(tracker.TrackedNodes()) != len(nodeIDs) {
				return false, fmt.Errorf(
					"expected nodes %v to stay marked draining, got %d",
					nodeIDs, len(tracker.TrackedNodes()))
			}
			return true, nil
		}),
	))

	for _, nodeID := range nodeIDs {
		must.MapContainsKey(t, tracker.TrackedNodes(), nodeID)
	}
}
