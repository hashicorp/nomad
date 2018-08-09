package base

import (
	"golang.org/x/net/context"

	"github.com/golang/protobuf/ptypes/empty"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/drivers/base/proto"
	"google.golang.org/grpc"
)

func LaunchDriver(name string, fac DriverFactory) error {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{},
		Plugins: map[string]plugin.Plugin{
			name: &DriverPlugin{impl: fac(nil)},
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

func (p *DriverPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterDriverServer(s, &baseDriver{
		impl:   p.impl,
		broker: broker,
	})
	return nil
}

type baseDriver struct {
	broker *plugin.GRPCBroker
	impl   Driver
}

func (b *baseDriver) RecoverTask(ctx context.Context, req *proto.RecoverTaskRequest) (*empty.Empty, error) {
	return nil, b.impl.RecoverTask(taskHandleFromProto(req.Handle))
}

func (b *baseDriver) StartTask(ctx context.Context, req *proto.StartTaskRequest) (*proto.StartTaskResponse, error) {
	handle, err := b.impl.StartTask(taskConfigFromProto(req.Task))
	if err != nil {
		return nil, err
	}

	return &proto.StartTaskResponse{
		Handle: taskHandleToProto(handle),
	}, nil
}
