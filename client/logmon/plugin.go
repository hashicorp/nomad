package logmon

import (
	"context"
	"os"
	"os/exec"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/logmon/proto"
	"github.com/hashicorp/nomad/plugins/base"
	"google.golang.org/grpc"
)

// LaunchLogMon launches a new logmon or reattaches to an existing one.
// TODO: Integrate with base plugin loader
func LaunchLogMon(logger hclog.Logger, reattachConfig *plugin.ReattachConfig) (LogMon, *plugin.Client, error) {
	logger = logger.Named("logmon")
	bin, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}

	conf := &plugin.ClientConfig{
		HandshakeConfig: base.Handshake,
		Plugins: map[string]plugin.Plugin{
			"logmon": &Plugin{},
		},
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
		Logger: logger,
	}

	// Only set one of Cmd or Reattach
	if reattachConfig == nil {
		conf.Cmd = exec.Command(bin, "logmon")
	} else {
		conf.Reattach = reattachConfig
	}

	client := plugin.NewClient(conf)

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
	return &logmonClient{
		doneCtx: ctx,
		client:  proto.NewLogMonClient(c),
	}, nil
}
