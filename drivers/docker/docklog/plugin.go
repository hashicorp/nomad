package docklog

import (
	"context"
	"os/exec"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/drivers/docker/docklog/proto"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/plugins/base"
	"google.golang.org/grpc"
)

// LaunchDocklog launches an instance of docklog
// TODO: Integrate with base plugin loader
func LaunchDocklog(logger hclog.Logger) (Docklog, *plugin.Client, error) {
	logger = logger.Named("docklog-launcher")
	bin, err := discover.NomadExecutable()
	if err != nil {
		return nil, nil, err
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: base.Handshake,
		Plugins: map[string]plugin.Plugin{
			"docklog": &Plugin{impl: NewDocklog(hclog.L().Named("docklog"))},
		},
		Cmd: exec.Command(bin, "docklog"),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
	})

	rpcClient, err := client.Client()
	if err != nil {
		return nil, nil, err
	}

	raw, err := rpcClient.Dispense("docklog")
	if err != nil {
		return nil, nil, err
	}

	l := raw.(Docklog)
	return l, client, nil

}

// Plugin is the go-plugin implementation
type Plugin struct {
	plugin.NetRPCUnsupportedPlugin
	impl Docklog
}

// GRPCServer registered the server side implementation with the grpc server
func (p *Plugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterDocklogServer(s, &docklogServer{
		impl:   p.impl,
		broker: broker,
	})
	return nil
}

// GRPCClient returns a client side implementation of the plugin
func (p *Plugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &docklogClient{client: proto.NewDocklogClient(c)}, nil
}
