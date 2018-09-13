package logmon

import (
	"context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/logmon/proto"
	"google.golang.org/grpc"
)

type Plugin struct {
	plugin.NetRPCUnsupportedPlugin
	impl LogMon
}

func NewPlugin(i LogMon) plugin.Plugin {
	return &Plugin{impl: i}
}

func (p *Plugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterLogMonServer(s, &logmonServer{
		impl:   p.impl,
		broker: broker,
	})
	return nil
}

func (p *Plugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &logmonClient{client: proto.NewLogMonClient(c)}, nil
}
