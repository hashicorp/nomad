// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package base

import (
	"context"
	"fmt"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base/proto"
)

// basePluginServer wraps a base plugin and exposes it via gRPC.
type basePluginServer struct {
	broker *plugin.GRPCBroker
	impl   BasePlugin
}

func (b *basePluginServer) PluginInfo(context.Context, *proto.PluginInfoRequest) (*proto.PluginInfoResponse, error) {
	resp, err := b.impl.PluginInfo()
	if err != nil {
		return nil, err
	}

	var ptype proto.PluginType
	switch resp.Type {
	case PluginTypeDriver:
		ptype = proto.PluginType_DRIVER
	case PluginTypeDevice:
		ptype = proto.PluginType_DEVICE
	default:
		return nil, fmt.Errorf("plugin is of unknown type: %q", resp.Type)
	}

	presp := &proto.PluginInfoResponse{
		Type:              ptype,
		PluginApiVersions: resp.PluginApiVersions,
		PluginVersion:     resp.PluginVersion,
		Name:              resp.Name,
	}

	return presp, nil
}

func (b *basePluginServer) ConfigSchema(context.Context, *proto.ConfigSchemaRequest) (*proto.ConfigSchemaResponse, error) {
	spec, err := b.impl.ConfigSchema()
	if err != nil {
		return nil, err
	}

	presp := &proto.ConfigSchemaResponse{
		Spec: spec,
	}

	return presp, nil
}

func (b *basePluginServer) SetConfig(ctx context.Context, req *proto.SetConfigRequest) (*proto.SetConfigResponse, error) {
	info, err := b.impl.PluginInfo()
	if err != nil {
		return nil, err
	}

	// Client configuration is filtered based on plugin type
	cfg := nomadConfigFromProto(req.GetNomadConfig())
	filteredCfg := new(AgentConfig)

	if cfg != nil {
		switch info.Type {
		case PluginTypeDriver:
			filteredCfg.Driver = cfg.Driver
		}
	}

	// Build the config request
	c := &Config{
		ApiVersion:   req.GetPluginApiVersion(),
		PluginConfig: req.GetMsgpackConfig(),
		AgentConfig:  filteredCfg,
	}

	// Set the config
	if err := b.impl.SetConfig(c); err != nil {
		return nil, fmt.Errorf("SetConfig failed: %w", err)
	}

	return &proto.SetConfigResponse{}, nil
}
