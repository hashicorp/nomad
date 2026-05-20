// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"context"

	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/mock"
)

type MockUniversalWatcher struct {
	mock.Mock
}

func (m *MockUniversalWatcher) Run(ctx context.Context) {
	m.Called()
}

// Watch the given check. If the check status enters a failing state, the
// task associated with the check will be restarted according to its check_restart
// policy via wr.
func (m *MockUniversalWatcher) Watch(checkID string, check *structs.ServiceCheck, wr serviceregistration.WorkloadRestarter) {
	m.Called(checkID, check, wr)
}

// Unwatch will cause the CheckWatcher to no longer monitor the check of given checkID.
func (m *MockUniversalWatcher) Unwatch(checkID string) {
	m.Called(checkID)
}
