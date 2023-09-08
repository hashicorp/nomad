// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package base

import (
	"context"
	"fmt"

	"github.com/hashicorp/nomad/helper/pluginutils/grpcutils"
	"github.com/hashicorp/nomad/plugins/base/proto"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

// BasePluginClient implements the client side of a remote base plugin, using
// gRPC to communicate to the remote plugin.
type BasePluginClient struct {
	Client proto.BasePluginClient

	// DoneCtx is closed when the plugin exits
	DoneCtx context.Context
}

func (b *BasePluginClient) PluginInfo() (*PluginInfoResponse, error) {
	presp, err := b.Client.PluginInfo(b.DoneCtx, &proto.PluginInfoRequest{})
	if err != nil {
		return nil, grpcutils.HandleGrpcErr(err, b.DoneCtx)
	}

	var ptype string
	switch presp.GetType() {
	case proto.PluginType_DRIVER:
		ptype = PluginTypeDriver
	case proto.PluginType_DEVICE:
		ptype = PluginTypeDevice
	default:
		return nil, fmt.Errorf("plugin is of unknown type: %q", presp.GetType().String())
	}

	resp := &PluginInfoResponse{
		Type:              ptype,
		PluginApiVersions: presp.GetPluginApiVersions(),
		PluginVersion:     presp.GetPluginVersion(),
		Name:              presp.GetName(),
	}

	return resp, nil
}

func (b *BasePluginClient) ConfigSchema() (*hclspec.Spec, error) {
	presp, err := b.Client.ConfigSchema(b.DoneCtx, &proto.ConfigSchemaRequest{})
	if err != nil {
		return nil, grpcutils.HandleGrpcErr(err, b.DoneCtx)
	}

	return presp.GetSpec(), nil
}

func (b *BasePluginClient) SetConfig(c *Config) error {
	// Send the config
	_, err := b.Client.SetConfig(b.DoneCtx, &proto.SetConfigRequest{
		MsgpackConfig:    c.PluginConfig,
		NomadConfig:      c.AgentConfig.toProto(),
		PluginApiVersion: c.ApiVersion,
	})

	return grpcutils.HandleGrpcErr(err, b.DoneCtx)
}
