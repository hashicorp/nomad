// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"sync"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
)

var _ ConfigAPI = (*MockConfigsAPI)(nil)

type MockConfigsAPI struct {
	logger hclog.Logger

	lock  sync.Mutex
	state struct {
		error   error
		entries map[string]api.ConfigEntry
	}
}

func NewMockConfigsAPI(l hclog.Logger) *MockConfigsAPI {
	return &MockConfigsAPI{
		logger: l.Named("mock_consul"),
		state: struct {
			error   error
			entries map[string]api.ConfigEntry
		}{entries: make(map[string]api.ConfigEntry)},
	}
}

// Set is a mock of ConfigAPI.Set
func (m *MockConfigsAPI) Set(entry api.ConfigEntry, w *api.WriteOptions) (bool, *api.WriteMeta, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.state.error != nil {
		return false, nil, m.state.error
	}

	m.state.entries[entry.GetName()] = entry

	return true, &api.WriteMeta{
		RequestTime: 1,
	}, nil
}

// SetError is a helper method for configuring an error that will be returned
// on future calls to mocked methods.
func (m *MockConfigsAPI) SetError(err error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.state.error = err
}
