// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package drainer

import (
	"context"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// This file contains helpers for testing. The raft shims make it hard to test
// the whole package behavior of the drainer. See also nomad/drainer_int_test.go
// for integration tests.

type MockJobWatcher struct {
	drainCh    chan *DrainRequest
	migratedCh chan []*structs.Allocation
	jobs       map[structs.NamespacedID]struct{}
	sync.Mutex
}

// RegisterJobs marks the job as being watched
func (m *MockJobWatcher) RegisterJobs(jobs []structs.NamespacedID) {
	m.Lock()
	defer m.Unlock()
	for _, job := range jobs {
		m.jobs[job] = struct{}{}
	}
}

// Drain returns the DrainRequest channel. Tests can send on this channel to
// simulate steps through the NodeDrainer watch loop. (Sending on this channel
// will block anywhere else.)
func (m *MockJobWatcher) Drain() <-chan *DrainRequest {
	return m.drainCh
}

// Migrated returns the channel of migrated allocations. Tests can send on this
// channel to simulate steps through the NodeDrainer watch loop. (Sending on
// this channel will block anywhere else.)
func (m *MockJobWatcher) Migrated() <-chan []*structs.Allocation {
	return m.migratedCh
}

type MockDeadlineNotifier struct {
	expiredCh <-chan []string
	nodes     map[string]struct{}
	sync.Mutex
}

// NextBatch returns the channel of expired nodes. Tests can send on this
// channel to simulate timer events in the NodeDrainer watch loop. (Sending on
// this channel will block anywhere else.)
func (m *MockDeadlineNotifier) NextBatch() <-chan []string {
	return m.expiredCh
}

// Remove removes the given node from being tracked for a deadline.
func (m *MockDeadlineNotifier) Remove(nodeID string) {
	m.Lock()
	defer m.Unlock()
	delete(m.nodes, nodeID)
}

// Watch marks the node as being watched; this mock throws out the timer in lieu
// of manully sending on the channel to avoid racy tests.
func (m *MockDeadlineNotifier) Watch(nodeID string, _ time.Time) {
	m.Lock()
	defer m.Unlock()
	m.nodes[nodeID] = struct{}{}
}

type MockRaftApplierShim struct {
	lock  sync.Mutex
	state *state.StateStore
}

// AllocUpdateDesiredTransition mocks a write to raft as a state store update
func (m *MockRaftApplierShim) AllocUpdateDesiredTransition(
	allocs map[string]*structs.DesiredTransition, evals []*structs.Evaluation) (uint64, error) {

	m.lock.Lock()
	defer m.lock.Unlock()

	index, _ := m.state.LatestIndex()
	index++
	err := m.state.UpdateAllocsDesiredTransitions(structs.MsgTypeTestSetup, index, allocs, evals)
	return index, err
}

// NodesDrainComplete mocks a write to raft as a state store update
func (m *MockRaftApplierShim) NodesDrainComplete(
	nodes []string, event *structs.NodeEvent) (uint64, error) {

	m.lock.Lock()
	defer m.lock.Unlock()

	index, _ := m.state.LatestIndex()
	index++

	updates := make(map[string]*structs.DrainUpdate, len(nodes))
	nodeEvents := make(map[string]*structs.NodeEvent, len(nodes))
	update := &structs.DrainUpdate{}
	for _, node := range nodes {
		updates[node] = update
		if event != nil {
			nodeEvents[node] = event
		}
	}
	now := time.Now().Unix()

	err := m.state.BatchUpdateNodeDrain(structs.MsgTypeTestSetup, index, now,
		updates, nodeEvents)

	return index, err
}

func testNodeDrainWatcher(t *testing.T) (*nodeDrainWatcher, *state.StateStore, *NodeDrainer) {
	t.Helper()
	store := state.TestStateStore(t)
	limiter := rate.NewLimiter(100.0, 100)
	logger := testlog.HCLogger(t)

	drainer := &NodeDrainer{
		enabled:          false,
		logger:           logger,
		nodes:            map[string]*drainingNode{},
		jobWatcher:       &MockJobWatcher{jobs: map[structs.NamespacedID]struct{}{}},
		deadlineNotifier: &MockDeadlineNotifier{nodes: map[string]struct{}{}},
		state:            store,
		queryLimiter:     limiter,
		raft:             &MockRaftApplierShim{state: store},
		batcher:          allocMigrateBatcher{},
	}

	w := NewNodeDrainWatcher(context.Background(), limiter, store, logger, drainer)
	drainer.nodeWatcher = w
	return w, store, drainer
}
