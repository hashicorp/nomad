package shared

import (
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

// MockPlugin is used for testing.
// Each function can be set as a closure to make assertions about how data
// is passed through the base plugin layer.
type MockPlugin struct {
	PluginInfoF   func() (*PluginInfoResponse, error)
	ConfigSchemaF func() (*hclspec.Spec, error)
	SetConfigF    func([]byte) error
}

func (p *MockPlugin) PluginInfo() (*PluginInfoResponse, error) { return p.PluginInfoF() }
func (p *MockPlugin) ConfigSchema() (*hclspec.Spec, error)     { return p.ConfigSchemaF() }
func (p *MockPlugin) SetConfig(data []byte) error              { return p.SetConfigF(data) }
