package base

import (
	"context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/drivers/base/proto"
	"google.golang.org/grpc"
)

func LaunchDriver(d Driver) error {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{},
		Plugins: map[string]plugin.Plugin{
			DriverGoPlugin: &DriverPlugin{impl: d},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
	return nil
}

type ServeConfig struct {
	EventRecorderRPCAddr string
	HandshakeConfig      *plugin.HandshakeConfig
	GRPCServer           func([]grpc.ServerOption) *grpc.Server
}

func Serve(cfg ServeConfig) error {

	return nil
}

type DriverFactory func(interface{}) Driver

type DriverPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	impl Driver
}

func NewDriverPlugin(d Driver) plugin.GRPCPlugin {
	return &DriverPlugin{impl: d}
}

func (p *DriverPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterDriverServer(s, &baseDriver{
		impl:   p.impl,
		broker: broker,
	})
	return nil
}

func (p *DriverPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &driverClient{client: proto.NewDriverClient(c)}, nil
}
