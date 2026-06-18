// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/mock"
)

type MockQueue struct {
	mock.Mock
}

// Start is a noop for the passthrough implementation
func (m *MockQueue) Start(context.Context) error { return nil }

func (m *MockQueue) Enqueue(e *structs.Evaluation) {
}

func (m *MockQueue) SetEnabled(bool, *state.StateStore) {}

func (m *MockQueue) Jobs(ns map[string]bool) structs.QueueJobsResponse {
	args := m.Called(ns)

	if args.Get(0) == nil {
		return structs.QueueJobsResponse{}
	}

	return args.Get(0).(structs.QueueJobsResponse)
}

func (m *MockQueue) Tenants() structs.QueueTenantsResponse {
	args := m.Called()

	if args.Get(0) == nil {
		return structs.QueueTenantsResponse{}
	}

	return args.Get(0).(structs.QueueTenantsResponse)
}
