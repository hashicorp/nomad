package base

import (
	"github.com/hashicorp/nomad/plugins/base/proto"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

// BasePlugin is the interface that all Nomad plugins must support.
type BasePlugin interface {
	// PluginInfo describes the type and version of a plugin.
	PluginInfo() (*PluginInfoResponse, error)

	// ConfigSchema returns the schema for parsing the plugins configuration.
	ConfigSchema() (*hclspec.Spec, error)

	// SetConfig is used to set the configuration by passing a MessagePack
	// encoding of it.
	SetConfig(data []byte, config *ClientAgentConfig) error
}

// PluginInfoResponse returns basic information about the plugin such that Nomad
// can decide whether to load the plugin or not.
type PluginInfoResponse struct {
	// Type returns the plugins type
	Type string

	// PluginApiVersion returns the version of the Nomad plugin API it is built
	// against.
	PluginApiVersion string

	// PluginVersion is the version of the plugin.
	PluginVersion string

	// Name is the plugins name.
	Name string
}

// ClientAgentConfig is the nomad client configuration sent to all plugins
type ClientAgentConfig struct {
	Driver *ClientDriverConfig
}

// ClientDriverConfig is the driver specific configuration for all driver plugins
type ClientDriverConfig struct {
	// ClientMaxPort is the upper range of the ports that the client uses for
	// communicating with plugin subsystems over loopback
	ClientMaxPort uint

	// ClientMinPort is the lower range of the ports that the client uses for
	// communicating with plugin subsystems over loopback
	ClientMinPort uint
}

func (c *ClientAgentConfig) toProto() *proto.NomadConfig {
	if c == nil {
		return nil
	}

	cfg := &proto.NomadConfig{}
	if c.Driver != nil {

		cfg.Driver = &proto.NomadDriverConfig{
			ClientMaxPort: uint32(c.Driver.ClientMaxPort),
			ClientMinPort: uint32(c.Driver.ClientMinPort),
		}
	}

	return cfg
}

func nomadConfigFromProto(pb *proto.NomadConfig) *ClientAgentConfig {
	if pb == nil {
		return nil
	}

	cfg := &ClientAgentConfig{}
	if pb.Driver != nil {
		cfg.Driver = &ClientDriverConfig{
			ClientMaxPort: uint(pb.Driver.ClientMaxPort),
			ClientMinPort: uint(pb.Driver.ClientMinPort),
		}
	}

	return cfg
}
