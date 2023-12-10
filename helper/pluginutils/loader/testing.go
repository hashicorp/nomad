// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package loader

import (
	"net"
	"sync"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/plugins/base"
)

// MockCatalog provides a mock PluginCatalog to be used for testing
type MockCatalog struct {
	DispenseF func(name, pluginType string, cfg *base.AgentConfig, logger log.Logger) (PluginInstance, error)
	ReattachF func(name, pluginType string, config *plugin.ReattachConfig) (PluginInstance, error)
	CatalogF  func() map[string][]*base.PluginInfoResponse
}

func (m *MockCatalog) Dispense(name, pluginType string, cfg *base.AgentConfig, logger log.Logger) (PluginInstance, error) {
	return m.DispenseF(name, pluginType, cfg, logger)
}

func (m *MockCatalog) Reattach(name, pluginType string, config *plugin.ReattachConfig) (PluginInstance, error) {
	return m.ReattachF(name, pluginType, config)
}

func (m *MockCatalog) Catalog() map[string][]*base.PluginInfoResponse {
	return m.CatalogF()
}

// MockInstance provides a mock PluginInstance to be used for testing
type MockInstance struct {
	InternalPlugin  bool
	KillF           func()
	ReattachConfigF func() (*plugin.ReattachConfig, bool)
	PluginF         func() interface{}
	ExitedF         func() bool
	ApiVersionF     func() string
}

func (m *MockInstance) Internal() bool                                 { return m.InternalPlugin }
func (m *MockInstance) Kill()                                          { m.KillF() }
func (m *MockInstance) ReattachConfig() (*plugin.ReattachConfig, bool) { return m.ReattachConfigF() }
func (m *MockInstance) Plugin() interface{}                            { return m.PluginF() }
func (m *MockInstance) Exited() bool                                   { return m.ExitedF() }
func (m *MockInstance) ApiVersion() string                             { return m.ApiVersionF() }

// MockBasicExternalPlugin returns a MockInstance that simulates an external
// plugin returning it has been exited after kill is called. It returns the
// passed inst as the plugin
func MockBasicExternalPlugin(inst interface{}, apiVersion string) *MockInstance {
	var killedLock sync.Mutex
	killed := pointer.Of(false)
	return &MockInstance{
		InternalPlugin: false,
		KillF: func() {
			killedLock.Lock()
			defer killedLock.Unlock()
			*killed = true
		},

		ReattachConfigF: func() (*plugin.ReattachConfig, bool) {
			return &plugin.ReattachConfig{
				Protocol: "tcp",
				Addr: &net.TCPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: 3200,
					Zone: "",
				},
				Pid: 1000,
			}, true
		},

		PluginF: func() interface{} {
			return inst
		},

		ExitedF: func() bool {
			killedLock.Lock()
			defer killedLock.Unlock()
			return *killed
		},

		ApiVersionF: func() string { return apiVersion },
	}
}
