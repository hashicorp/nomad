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
								Required: false,
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

type PluginInfoFn func() (*PluginInfoResponse, error)
type ConfigSchemaFn func() (*hclspec.Spec, error)
type SetConfigFn func(*Config) error

// MockPlugin is used for testing.
// Each function can be set as a closure to make assertions about how data
// is passed through the base plugin layer.
type MockPlugin struct {
	PluginInfoF   PluginInfoFn
	ConfigSchemaF ConfigSchemaFn
	SetConfigF    SetConfigFn
}

func (p *MockPlugin) PluginInfo() (*PluginInfoResponse, error) { return p.PluginInfoF() }
func (p *MockPlugin) ConfigSchema() (*hclspec.Spec, error)     { return p.ConfigSchemaF() }
func (p *MockPlugin) SetConfig(cfg *Config) error {
	return p.SetConfigF(cfg)
}

// Below are static implementations of the base plugin functions

// StaticInfo returns the passed PluginInfoResponse with no error
func StaticInfo(out *PluginInfoResponse) PluginInfoFn {
	return func() (*PluginInfoResponse, error) {
		return out, nil
	}
}

// StaticConfigSchema returns the passed Spec with no error
func StaticConfigSchema(out *hclspec.Spec) ConfigSchemaFn {
	return func() (*hclspec.Spec, error) {
		return out, nil
	}
}

// TestConfigSchema returns a ConfigSchemaFn that statically returns the
// TestSpec
func TestConfigSchema() ConfigSchemaFn {
	return StaticConfigSchema(TestSpec)
}

// NoopSetConfig is a noop implementation of set config
func NoopSetConfig() SetConfigFn {
	return func(_ *Config) error { return nil }
}
