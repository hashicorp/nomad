package base

import (
	"fmt"

	"golang.org/x/net/context"

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
		Type:             ptype,
		PluginApiVersion: resp.PluginApiVersion,
		PluginVersion:    resp.PluginVersion,
		Name:             resp.Name,
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
	// Set the config
	if err := b.impl.SetConfig(req.GetMsgpackConfig()); err != nil {
		return nil, fmt.Errorf("SetConfig failed: %v", err)
	}

	return &proto.SetConfigResponse{}, nil
}
