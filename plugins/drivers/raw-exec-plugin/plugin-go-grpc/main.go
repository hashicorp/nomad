package main

import (
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/drivers/raw-exec-plugin/plugin-go-grpc/raw_exec"
	"github.com/hashicorp/nomad/plugins/drivers/raw-exec-plugin/shared"
)

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: shared.Handshake,
		Plugins: map[string]plugin.Plugin{
			"raw_exec": &shared.RawExecPlugin{Impl: &raw_exec.RawExec{}},
		},

		GRPCServer: plugin.DefaultGRPCServer,
	})
}
