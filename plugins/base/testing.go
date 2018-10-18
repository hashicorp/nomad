package base

import (
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

var (
	// TestSpec is an hcl Spec for testing
	TestSpec = &hclspec.Spec{
		Block: &hclspec.Spec_Object{
			Object: &hclspec.Object{
				Attributes: map[string]*hclspec.Spec{
					"foo": {
						Block: &hclspec.Spec_Attr{
							Attr: &hclspec.Attr{
								Type:     "string",
								Required: false,
							},
						},
					},
					"bar": {
						Block: &hclspec.Spec_Attr{
							Attr: &hclspec.Attr{
								Type:     "number",
								Required: true,
							},
						},
					},
					"baz": {
						Block: &hclspec.Spec_Attr{
							Attr: &hclspec.Attr{
								Type: "bool",
							},
						},
					},
				},
			},
		},
	}
)

// TestConfig is used to decode a config from the TestSpec
type TestConfig struct {
	Foo string `cty:"foo" codec:"foo"`
	Bar int64  `cty:"bar" codec:"bar"`
	Baz bool   `cty:"baz" codec:"baz"`
}

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
