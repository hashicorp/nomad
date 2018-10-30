package loader

import (
	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base"
)

// MockCatalog provides a mock PluginCatalog to be used for testing
type MockCatalog struct {
	DispenseF func(name, pluginType string, cfg *base.ClientAgentConfig, logger log.Logger) (PluginInstance, error)
	ReattachF func(name, pluginType string, config *plugin.ReattachConfig) (PluginInstance, error)
	CatalogF  func() map[string][]*base.PluginInfoResponse
}

func (m *MockCatalog) Dispense(name, pluginType string, cfg *base.ClientAgentConfig, logger log.Logger) (PluginInstance, error) {
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
}

func (m *MockInstance) Internal() bool                                 { return m.InternalPlugin }
func (m *MockInstance) Kill()                                          { m.KillF() }
func (m *MockInstance) ReattachConfig() (*plugin.ReattachConfig, bool) { return m.ReattachConfigF() }
func (m *MockInstance) Plugin() interface{}                            { return m.PluginF() }
func (m *MockInstance) Exited() bool                                   { return m.ExitedF() }
