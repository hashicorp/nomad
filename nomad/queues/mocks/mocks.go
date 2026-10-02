// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package mocks

import (
	"context"

	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/mock"
)

type MockQueueManager struct {
	mock.Mock
}

func (m *MockQueueManager) SetEnabled(enabled bool, state *state.StateStore) {
	m.Called(enabled, state)
}
func (m *MockQueueManager) Enqueue(e *structs.Evaluation) {
	m.Called(e)
}
func (m *MockQueueManager) Dequeue(job *structs.Job) *structs.Evaluation {
	if args := m.Called(job); args.Get(0) != nil {
		return args.Get(0).(*structs.Evaluation)
	}
	return nil
}
func (m *MockQueueManager) Queue(pool string) queue.QueueRunner {
	// Queue on the real manager should never return nil
	return m.Called(pool).Get(0).(queue.QueueRunner)
}
func (m *MockQueueManager) UpdateQueue(pool *structs.NodePool) error {
	if args := m.Called(pool); args.Get(0) != nil {
		return args.Get(0).(error)
	}
	return nil
}

type MockBroker struct {
	mock.Mock
}

func (m *MockBroker) Enqueue(e *structs.Evaluation) {
	m.Called(e)
}

type MockQueueRunner struct {
	mock.Mock

	Name string
}

func (m *MockQueueRunner) Type() structs.BatchQueueType {
	if m.Name != "" {
		return structs.BatchQueueType(m.Name)
	}
	return "test"
}

func (m *MockQueueRunner) Start(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockQueueRunner) Stop() {
	m.Called()
}

func (m *MockQueueRunner) Restore(e *structs.Evaluation, j *structs.Job) error {
	// mock-call IDs for cleaner error outputs
	m.Called(e.ID, j.ID)
	return nil
}

func (m *MockQueueRunner) Enqueue(e *structs.Evaluation, j *structs.Job) {
	// mock-call IDs for cleaner error outputs
	m.Called(e.ID, j.ID)
}

func (m *MockQueueRunner) Dequeue(id structs.NamespacedID) {
	m.Called(id)
}

func (m *MockQueueRunner) Jobs(sortOrder structs.SortOrder) *queue.WorkloadIter {
	args := m.Called(sortOrder)

	if args.Get(0) == nil {
		return &queue.WorkloadIter{}
	}

	return args.Get(0).(*queue.WorkloadIter)
}

func (m *MockQueueRunner) Tenants() structs.QueueTenantsResponse {
	args := m.Called()

	if args.Get(0) == nil {
		return structs.QueueTenantsResponse{}
	}

	return args.Get(0).(structs.QueueTenantsResponse)
}

type MockQueue struct {
	mock.Mock
}

func (m *MockQueue) NewWorkload(e *structs.Evaluation, j *structs.Job) (queue.Workload, bool) {
	args := m.Called(e, j)
	wl, _ := args.Get(0).(queue.Workload)
	return wl, args.Bool(1)
}

func (m *MockQueue) Push(w queue.Workload) {
	m.Called(w)
}

func (m *MockQueue) Pop() (queue.Workload, bool) {
	args := m.Called()
	wl, _ := args.Get(0).(queue.Workload)
	return wl, args.Bool(1)
}

func (m *MockQueue) Get(id structs.NamespacedID) (queue.Workload, bool) {
	args := m.Called(id)
	wl, _ := args.Get(0).(queue.Workload)
	return wl, args.Bool(1)
}

func (m *MockQueue) Remove(id structs.NamespacedID) (queue.Workload, bool) {
	args := m.Called(id)
	wl, _ := args.Get(0).(queue.Workload)
	return wl, args.Bool(1)
}

func (m *MockQueue) Swap(w queue.Workload) (queue.Workload, bool) {
	args := m.Called(w)
	wl, _ := args.Get(0).(queue.Workload)
	return wl, args.Bool(1)
}

func (m *MockQueue) Type() structs.BatchQueueType {
	args := m.Called()
	return args.Get(0).(structs.BatchQueueType)
}
