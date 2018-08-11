package base

import (
	ccontext "context"

	"golang.org/x/net/context"

	"github.com/golang/protobuf/ptypes/empty"
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

func (p *DriverPlugin) GRPCClient(ctx ccontext.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &driverClient{client: proto.NewDriverClient(c)}, nil
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

func (b *baseDriver) WaitTask(ctx context.Context, req *proto.WaitTaskRequest) (*proto.WaitTaskResponse, error) {
	ch := b.impl.WaitTask(req.TaskId)
	result := <-ch
	var errStr string
	if result.Err != nil {
		errStr = result.Err.Error()
	}
	return &proto.WaitTaskResponse{
		ExitCode: int32(result.ExitCode),
		Signal:   int32(result.Signal),
		Err:      errStr,
	}, nil
}
