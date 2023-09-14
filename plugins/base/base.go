// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package base

import (
	"github.com/hashicorp/nomad/client/lib/cpustats"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/helper"
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
	SetConfig(c *Config) error
}

// PluginInfoResponse returns basic information about the plugin such that Nomad
// can decide whether to load the plugin or not.
type PluginInfoResponse struct {
	// Type returns the plugins type
	Type string

	// PluginApiVersions returns the versions of the Nomad plugin API that the
	// plugin supports.
	PluginApiVersions []string

	// PluginVersion is the version of the plugin.
	PluginVersion string

	// Name is the plugins name.
	Name string
}

// Config contains the configuration for the plugin.
type Config struct {
	// ApiVersion is the negotiated plugin API version to use.
	ApiVersion string

	// PluginConfig is the MessagePack encoding of the plugins user
	// configuration.
	PluginConfig []byte

	// AgentConfig is the Nomad agents configuration as applicable to plugins
	AgentConfig *AgentConfig
}

// AgentConfig is the Nomad agent's configuration sent to all plugins
type AgentConfig struct {
	Driver *ClientDriverConfig
}

// Compute gets the basic cpu compute availablility necessary for drivers.
func (ac *AgentConfig) Compute() cpustats.Compute {
	if ac == nil || ac.Driver == nil || ac.Driver.Topology == nil {
		return cpustats.Compute{}
	}
	return ac.Driver.Topology.Compute()
}

// ClientDriverConfig is the driver specific configuration for all driver plugins
type ClientDriverConfig struct {
	// ClientMaxPort is the upper range of the ports that the client uses for
	// communicating with plugin subsystems over loopback
	ClientMaxPort uint

	// ClientMinPort is the lower range of the ports that the client uses for
	// communicating with plugin subsystems over loopback
	ClientMinPort uint

	// Topology is the system hardware topology that is the result of scanning
	// hardware combined with client configuration.
	Topology *numalib.Topology
}

func (c *AgentConfig) toProto() *proto.NomadConfig {
	if c == nil {
		return nil
	}
	cfg := &proto.NomadConfig{}
	if c.Driver != nil {
		cfg.Driver = &proto.NomadDriverConfig{
			ClientMaxPort: uint32(c.Driver.ClientMaxPort),
			ClientMinPort: uint32(c.Driver.ClientMinPort),
			Topology:      nomadTopologyToProto(c.Driver.Topology),
		}
	}
	return cfg
}

func nomadConfigFromProto(pb *proto.NomadConfig) *AgentConfig {
	if pb == nil {
		return nil
	}
	cfg := &AgentConfig{}
	if pb.Driver != nil {
		cfg.Driver = &ClientDriverConfig{
			ClientMaxPort: uint(pb.Driver.ClientMaxPort),
			ClientMinPort: uint(pb.Driver.ClientMinPort),
			Topology:      nomadTopologyFromProto(pb.Driver.Topology),
		}
	}
	return cfg
}

func nomadTopologyFromProto(pb *proto.ClientTopology) *numalib.Topology {
	if pb == nil {
		return nil
	}
	return &numalib.Topology{
		NodeIDs:                idset.FromFunc(pb.NodeIds, func(i uint32) hw.NodeID { return hw.NodeID(i) }),
		Distances:              nomadTopologyDistancesFromProto(pb.Distances),
		Cores:                  nomadTopologyCoresFromProto(pb.Cores),
		OverrideTotalCompute:   hw.MHz(pb.OverrideTotalCompute),
		OverrideWitholdCompute: hw.MHz(pb.OverrideWitholdCompute),
	}
}

func nomadTopologyDistancesFromProto(pb *proto.ClientTopologySLIT) numalib.SLIT {
	if pb == nil {
		return nil
	}
	size := int(pb.Dimension)
	slit := make(numalib.SLIT, size)
	for row := 0; row < size; row++ {
		slit[row] = make([]numalib.Cost, size)
		for col := 0; col < size; col++ {
			index := row*size + col
			slit[row][col] = numalib.Cost(pb.Values[index])
		}
	}
	return slit
}

func nomadTopologyCoresFromProto(pb []*proto.ClientTopologyCore) []numalib.Core {
	if len(pb) == 0 {
		return nil
	}
	return helper.ConvertSlice(pb, func(pbcore *proto.ClientTopologyCore) numalib.Core {
		return numalib.Core{
			SocketID:   hw.SocketID(pbcore.SocketId),
			NodeID:     hw.NodeID(pbcore.NodeId),
			ID:         hw.CoreID(pbcore.CoreId),
			Grade:      numalib.CoreGrade(pbcore.CoreGrade),
			Disable:    pbcore.Disable,
			BaseSpeed:  hw.MHz(pbcore.BaseSpeed),
			MaxSpeed:   hw.MHz(pbcore.MaxSpeed),
			GuessSpeed: hw.MHz(pbcore.GuessSpeed),
		}
	})
}

func nomadTopologyToProto(top *numalib.Topology) *proto.ClientTopology {
	if top == nil {
		return nil
	}
	return &proto.ClientTopology{
		NodeIds:                helper.ConvertSlice(top.NodeIDs.Slice(), func(id hw.NodeID) uint32 { return uint32(id) }),
		Distances:              nomadTopologyDistancesToProto(top.Distances),
		Cores:                  nomadTopologyCoresToProto(top.Cores),
		OverrideTotalCompute:   uint64(top.OverrideTotalCompute),
		OverrideWitholdCompute: uint64(top.OverrideWitholdCompute),
	}
}

func nomadTopologyDistancesToProto(slit numalib.SLIT) *proto.ClientTopologySLIT {
	dimension := len(slit)
	values := make([]uint32, 0, dimension)
	for row := 0; row < dimension; row++ {
		for col := 0; col < dimension; col++ {
			values = append(values, uint32(slit[row][col]))
		}
	}
	return &proto.ClientTopologySLIT{
		Dimension: uint32(dimension),
		Values:    values,
	}
}

func nomadTopologyCoresToProto(cores []numalib.Core) []*proto.ClientTopologyCore {
	if len(cores) == 0 {
		return nil
	}
	return helper.ConvertSlice(cores, func(core numalib.Core) *proto.ClientTopologyCore {
		return &proto.ClientTopologyCore{
			SocketId:   uint32(core.SocketID),
			NodeId:     uint32(core.NodeID),
			CoreId:     uint32(core.ID),
			CoreGrade:  bool(core.Grade),
			Disable:    core.Disable,
			BaseSpeed:  uint64(core.BaseSpeed),
			MaxSpeed:   uint64(core.MaxSpeed),
			GuessSpeed: uint64(core.GuessSpeed),
		}
	})
}
