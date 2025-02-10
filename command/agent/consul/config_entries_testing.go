// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"sync"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/lib/lang"
)

var _ ConfigAPI = (*MockConfigsAPI)(nil)

type MockConfigsAPI struct {
	logger hclog.Logger

	lock  sync.Mutex
	state struct {
		error   error
		entries map[string]lang.Pair[api.ConfigEntry, *api.WriteOptions]
	}
}

func NewMockConfigsAPI(l hclog.Logger) *MockConfigsAPI {
	return &MockConfigsAPI{
		logger: l.Named("mock_consul"),
		state: struct {
			error   error
			entries map[string]lang.Pair[api.ConfigEntry, *api.WriteOptions]
		}{entries: make(map[string]lang.Pair[api.ConfigEntry, *api.WriteOptions])},
	}
}

// Set is a mock of ConfigAPI.Set
func (m *MockConfigsAPI) Set(entry api.ConfigEntry, w *api.WriteOptions) (bool, *api.WriteMeta, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.state.error != nil {
		return false, nil, m.state.error
	}

	m.state.entries[entry.GetName()] = lang.Pair[api.ConfigEntry, *api.WriteOptions]{First: entry, Second: w}

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

// GetEntry is a helper method so that test can verify what's been written
func (m *MockConfigsAPI) GetEntry(kind string) (api.ConfigEntry, *api.WriteOptions) {
	m.lock.Lock()
	defer m.lock.Unlock()
	entry := m.state.entries[kind]
	return entry.First, entry.Second
}
