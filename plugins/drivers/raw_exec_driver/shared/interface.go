// Package shared contains shared data between the host and plugins.
package shared

import (
	"context"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/drivers/raw_exec_driver/proto"
	"google.golang.org/grpc"
)

var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "BASIC_PLUGIN",
	MagicCookieValue: "hello",
}

var PluginMap = map[string]plugin.Plugin{
	"raw_exec": &RawExecPlugin{},
}

type RawExec interface {
	NewStart(*proto.ExecContext, *proto.TaskInfo) (*proto.StartResponse, error)
}

type RawExecPlugin struct {
	Impl RawExec
}

func (p *RawExecPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &RPCServer{Impl: p.Impl}, nil
}

func (*RawExecPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &RPCClient{client: c}, nil
}

func (p *RawExecPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterRawExecServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

func (p *RawExecPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: proto.NewRawExecClient(c)}, nil
}
