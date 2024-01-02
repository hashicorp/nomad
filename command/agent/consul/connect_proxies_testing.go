// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

// ConnectProxies implements SupportedProxiesAPI by mocking the Consul Agent API.
type MockSupportedProxiesAPI struct {
	Value map[string][]string
	Error error
}

func (m MockSupportedProxiesAPI) Proxies() (map[string][]string, error) {
	return m.Value, m.Error
}
