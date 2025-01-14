// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package logmon

import (
	"context"
	"os"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/logmon/proto"
	"github.com/hashicorp/nomad/plugins/base"
	"google.golang.org/grpc"
)

var bin = getBin()

func getBin() string {
	b, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return b
}

// LaunchLogMon launches a new logmon or reattaches to an existing one.
// TODO: Integrate with base plugin loader
func LaunchLogMon(logger hclog.Logger, reattachConfig *plugin.ReattachConfig) (LogMon, *plugin.Client, error) {
	logger = logger.Named("logmon")
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

	// Note: Similar to reattaching to executors, Go-plugin uses localhost
	// ports on Windows. On reattach, it may attach to another process
	// listening on that port. We should validate it is actually a plugin.
	if conf.Reattach != nil {
		if err := rpcClient.Ping(); err != nil {
			logger.Warn("failed to ping plugin process during reattach", "error", err)
			return nil, nil, err
		}
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
