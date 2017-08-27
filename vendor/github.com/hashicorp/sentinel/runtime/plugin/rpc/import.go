package rpc

import (
	"net/rpc"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/sentinel/proto/go"
	"github.com/hashicorp/sentinel/runtime/plugin"
	"google.golang.org/grpc"
)

// ImportPlugin is the plugin.Plugin implementation to serve plugin.Import.
type ImportPlugin struct {
	F func() plugin.Import
}

func (p *ImportPlugin) Server(b *goplugin.MuxBroker) (interface{}, error) {
	return &ImportServer{Broker: b, Import: p.F()}, nil
}

func (p *ImportPlugin) Client(
	b *goplugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &Import{Broker: b, Client: c}, nil
}

func (p *ImportPlugin) GRPCServer(s *grpc.Server) error {
	proto.RegisterImportServer(s, &ImportGRPCServer{F: p.F})
	return nil
}

func (p *ImportPlugin) GRPCClient(c *grpc.ClientConn) (interface{}, error) {
	return &ImportGRPCClient{Client: proto.NewImportClient(c)}, nil
}
