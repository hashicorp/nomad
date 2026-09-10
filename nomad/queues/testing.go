// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"

	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/mock"
)

// WithQueue allows passing in a queue in the constructor (for testing)
func WithQueue(pool string, q queue.Queue) QueueMgrOpt {
	return func(b *BatchQueueManager) {
		b.qk.Set(pool, q, false)
	}
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

func (m *MockQueue) Dequeue(j *structs.Job) *structs.Evaluation {
	args := m.Called(j)

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
