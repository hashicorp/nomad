package drivers

import (
	"context"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base"
	baseproto "github.com/hashicorp/nomad/plugins/base/proto"
	"github.com/hashicorp/nomad/plugins/drivers/proto"
	"google.golang.org/grpc"
)

// PluginDriver wraps a DriverPlugin and implements go-plugins GRPCPlugin
// interface to expose the the interface over gRPC
type PluginDriver struct {
	plugin.NetRPCUnsupportedPlugin
	impl   DriverPlugin
	logger hclog.Logger
}

func NewDriverPlugin(d DriverPlugin, logger hclog.Logger) plugin.GRPCPlugin {
	return &PluginDriver{
		impl:   d,
		logger: logger.Named("driver_plugin"),
	}
}

func (p *PluginDriver) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterDriverServer(s, &driverPluginServer{
		impl:   p.impl,
		broker: broker,
		logger: p.logger,
	})
	return nil
}

func (p *PluginDriver) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &driverPluginClient{
		BasePluginClient: &base.BasePluginClient{
			DoneCtx: ctx,
			Client:  baseproto.NewBasePluginClient(c),
		},
		client:  proto.NewDriverClient(c),
		logger:  p.logger,
		doneCtx: ctx,
	}, nil
}
