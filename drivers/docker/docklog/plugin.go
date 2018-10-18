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

const pluginName = "docker_logger"

// LaunchDockerLogger launches an instance of DockerLogger
// TODO: Integrate with base plugin loader
func LaunchDockerLogger(logger hclog.Logger) (DockerLogger, *plugin.Client, error) {
	logger = logger.Named(pluginName)
	bin, err := discover.NomadExecutable()
	if err != nil {
		return nil, nil, err
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: base.Handshake,
		Plugins: map[string]plugin.Plugin{
			pluginName: &Plugin{impl: NewDockerLogger(hclog.L().Named(pluginName))},
		},
		Cmd: exec.Command(bin, pluginName),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
	})

	rpcClient, err := client.Client()
	if err != nil {
		return nil, nil, err
	}

	raw, err := rpcClient.Dispense(pluginName)
	if err != nil {
		return nil, nil, err
	}

	l := raw.(DockerLogger)
	return l, client, nil

}

// Plugin is the go-plugin implementation
type Plugin struct {
	plugin.NetRPCUnsupportedPlugin
	impl DockerLogger
}

// GRPCServer registered the server side implementation with the grpc server
func (p *Plugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterDockerLoggerServer(s, &dockerLoggerServer{
		impl:   p.impl,
		broker: broker,
	})
	return nil
}

// GRPCClient returns a client side implementation of the plugin
func (p *Plugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &dockerLoggerClient{client: proto.NewDockerLoggerClient(c)}, nil
}
