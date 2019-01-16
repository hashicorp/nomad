package logmon

import (
	"context"
	"os/exec"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/logmon/proto"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/plugins/base"
	"google.golang.org/grpc"
)

// LaunchLogMon an instance of logmon
// TODO: Integrate with base plugin loader
func LaunchLogMon(logger hclog.Logger, reattachConfig *plugin.ReattachConfig) (LogMon, *plugin.Client, error) {
	logger = logger.Named("logmon")
	bin, err := discover.NomadExecutable()
	if err != nil {
		return nil, nil, err
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: base.Handshake,
		Reattach:        reattachConfig,
		Plugins: map[string]plugin.Plugin{
			"logmon": &Plugin{},
		},
		Cmd: exec.Command(bin, "logmon"),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
		Logger: logger,
	})

	rpcClient, err := client.Client()
	if err != nil {
		return nil, nil, err
	}

	raw, err := rpcClient.Dispense("logmon")
	if err != nil {
		return nil, nil, err
	}

	l := raw.(LogMon)
	return l, client, nil
}

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
