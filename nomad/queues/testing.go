// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/mock"
)

type MockQueue struct {
	mock.Mock
}

func (m *MockQueue) Type() structs.BatchQueueType {
	return "test"
}

// Start is a noop for the passthrough implementation
func (m *MockQueue) Start(context.Context) error { return nil }

func (m *MockQueue) Stop() {
}

func (m *MockQueue) Enqueue(e *structs.Evaluation) {
	m.Called(e)
}

func (m *MockQueue) Jobs(sortOrder structs.SortOrder) *WorkloadIter {
	args := m.Called(sortOrder)

	if args.Get(0) == nil {
		return &WorkloadIter{}
	}

	return args.Get(0).(*WorkloadIter)
}

func (m *MockQueue) Tenants() structs.QueueTenantsResponse {
	args := m.Called()

	if args.Get(0) == nil {
		return structs.QueueTenantsResponse{}
	}

	return args.Get(0).(structs.QueueTenantsResponse)
}
