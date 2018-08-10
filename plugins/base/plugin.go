package shared

import (
	"golang.org/x/net/context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base/proto"
	"google.golang.org/grpc"
)

const (
	// PluginTypeBase implements the base plugin driver interface
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

type PluginBase struct {
	plugin.NetRPCUnsupportedPlugin
	impl BasePlugin
}

func (p *PluginBase) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterBasePluginServer(s, &basePluginServer{impl: p.impl})
	return nil
}

func (p *PluginBase) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &basePluginClient{client: proto.NewBasePluginClient(c)}, nil
}
