package shared

import (
	"fmt"

	"github.com/hashicorp/nomad/plugins/base/proto"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"golang.org/x/net/context"
)

// basePluginClient implements the client side of a remote base plugin, using
// gRPC to communicate to the remote plugin.
type basePluginClient struct {
	client proto.BasePluginClient
}

func (b *basePluginClient) PluginInfo() (*PluginInfoResponse, error) {
	presp, err := b.client.PluginInfo(context.Background(), &proto.PluginInfoRequest{})
	if err != nil {
		return nil, err
	}

	var ptype string
	switch presp.GetType() {
	case proto.PluginType_BASE:
		ptype = PluginTypeBase
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

func (b *basePluginClient) ConfigSchema() (*hclspec.Spec, error) {
	presp, err := b.client.ConfigSchema(context.Background(), &proto.ConfigSchemaRequest{})
	if err != nil {
		return nil, err
	}

	return presp.GetSpec(), nil
}

func (b *basePluginClient) SetConfig(data []byte) error {
	// Send the config
	_, err := b.client.SetConfig(context.Background(), &proto.SetConfigRequest{
		MsgpackConfig: data,
	})

	return err
}
