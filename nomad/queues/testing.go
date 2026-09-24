// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"

	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/mock"
)

type QueueManager interface {
	SetEnabled(bool, *state.StateStore)
	Enqueue(*structs.Evaluation)
	Dequeue(*structs.Job) *structs.Evaluation
	Queue(string) queue.Queue
	UpdateQueue(*structs.NodePool) error
}

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
func (m *MockQueueManager) Queue(pool string) queue.Queue {
	// Queue on the real manager should never return nil
	return m.Called(pool).Get(0).(queue.Queue)
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
	m.Called(e.JobID) // concrete type for cleaner assertions
}

type MockQueue struct {
	mock.Mock

	name string
}

func (m *MockQueue) Type() structs.BatchQueueType {
	if m.name != "" {
		return structs.BatchQueueType(m.name)
	}
	return "test"
}

func (m *MockQueue) Start(context.Context) error {
	m.Called()
	return nil
}

func (m *MockQueue) Stop() {
	m.Called()
}

func (m *MockQueue) Restore(e *structs.Evaluation, j *structs.Job) error {
	// mock-call IDs for cleaner error outputs
	m.Called(e.ID, j.ID)
	return nil
}

func (m *MockQueue) Enqueue(e *structs.Evaluation, j *structs.Job) {
	// mock-call IDs for cleaner error outputs
	m.Called(e.ID, j.ID)
}

func (m *MockQueue) Dequeue(id structs.NamespacedID) *structs.Evaluation {
	args := m.Called(id)

	return args.Get(0).(*structs.Evaluation)
}

func (m *MockQueue) Jobs(sortOrder structs.SortOrder) *queue.WorkloadIter {
	args := m.Called(sortOrder)

	if args.Get(0) == nil {
		return &queue.WorkloadIter{}
	}

	return args.Get(0).(*queue.WorkloadIter)
}

func (m *MockQueue) Tenants() structs.QueueTenantsResponse {
	args := m.Called()

	if args.Get(0) == nil {
		return structs.QueueTenantsResponse{}
	}

	return args.Get(0).(structs.QueueTenantsResponse)
}
