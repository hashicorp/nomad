package base

import (
	"golang.org/x/net/context"

	"github.com/golang/protobuf/ptypes"
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

type DriverFactory func(TaskRecorder) Driver

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

func (d *baseDriver) CreateTask(ctx context.Context, req *proto.CreateTaskRequest) (*proto.CreateTaskResponse, error) {
	task := TaskConfig{
		Name: req.Task.Name,
		User: req.Task.User,
		Env:  req.Task.Env,
	}
	// More struct munging needed
	tid, err := d.impl.CreateTask(task)
	if err != nil {
		return nil, err
	}
	return &proto.CreateTaskResponse{TaskId: tid}, nil
}

func (b *baseDriver) RecoverTask(ctx context.Context, req *proto.RecoverTaskRequest) (*empty.Empty, error) {
	events := []TaskEvent{}
	for _, e := range req.Events {
		timestamp, err := ptypes.Timestamp(e.Timestamp)
		if err != nil {
			return nil, err
		}
		event := TaskEvent{
			Type:        TaskEventTypeFromString(e.EventType),
			Timestamp:   timestamp,
			Driver:      e.Driver,
			Description: e.Description,
			Attrs:       e.Attrs,
		}

		events = append(events, event)
	}
	err := b.impl.RecoverTask(req.TaskId, events)
	return nil, err
}
