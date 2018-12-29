package base

import (
	"context"
	"fmt"

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
		return nil, err
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
		Type:             ptype,
		PluginApiVersion: presp.GetPluginApiVersion(),
		PluginVersion:    presp.GetPluginVersion(),
		Name:             presp.GetName(),
	}

	return resp, nil
}

func (b *BasePluginClient) ConfigSchema() (*hclspec.Spec, error) {
	presp, err := b.Client.ConfigSchema(b.DoneCtx, &proto.ConfigSchemaRequest{})
	if err != nil {
		return nil, err
	}

	return presp.GetSpec(), nil
}

func (b *BasePluginClient) SetConfig(data []byte, config *ClientAgentConfig) error {
	// Send the config
	_, err := b.Client.SetConfig(b.DoneCtx, &proto.SetConfigRequest{
		MsgpackConfig: data,
		NomadConfig:   config.toProto(),
	})

	return err
}
