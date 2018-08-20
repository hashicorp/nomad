package base

import (
	"context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base/proto"
	"google.golang.org/grpc"
)

const (
	// PluginTypeBase implements the base plugin interface
	PluginTypeBase = "base"

	// PluginTypeDriver implements the driver plugin interface
	PluginTypeDriver = "driver"

	// PluginTypeDevice implements the device plugin interface
	PluginTypeDevice = "device"
)

var (
	// Handshake is a common handshake that is shared by all plugins and Nomad.
	Handshake = plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "NOMAD_PLUGIN_MAGIC_COOKIE",
		MagicCookieValue: "e4327c2e01eabfd75a8a67adb114fb34a757d57eee7728d857a8cec6e91a7255",
	}
)

// PluginBase is wraps a BasePlugin and implements go-plugins GRPCPlugin
// interface to expose the interface over gRPC.
type PluginBase struct {
	plugin.NetRPCUnsupportedPlugin
	Impl BasePlugin
}

func (p *PluginBase) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterBasePluginServer(s, &basePluginServer{
		impl:   p.Impl,
		broker: broker,
	})
	return nil
}

func (p *PluginBase) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &BasePluginClient{Client: proto.NewBasePluginClient(c)}, nil
}
