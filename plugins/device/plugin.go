package device

import (
	"context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base"
	bproto "github.com/hashicorp/nomad/plugins/base/proto"
	"github.com/hashicorp/nomad/plugins/device/proto"
	"google.golang.org/grpc"
)

// PluginDevice is wraps a DevicePlugin and implements go-plugins GRPCPlugin
// interface to expose the interface over gRPC.
type PluginDevice struct {
	plugin.NetRPCUnsupportedPlugin
	Impl DevicePlugin
}

func (p *PluginDevice) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterDevicePluginServer(s, &devicePluginServer{
		impl:   p.Impl,
		broker: broker,
	})
	return nil
}

func (p *PluginDevice) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &devicePluginClient{
		client: proto.NewDevicePluginClient(c),
		BasePluginClient: &base.BasePluginClient{
			Client: bproto.NewBasePluginClient(c),
		},
	}, nil
}
