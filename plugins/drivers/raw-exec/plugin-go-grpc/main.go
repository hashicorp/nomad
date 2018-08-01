package main

import (
	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/plugins/drivers/raw-exec/plugin-go-grpc/raw_exec"
	"github.com/hashicorp/nomad/plugins/drivers/raw-exec/shared"
)

func main() {
	driverCtx := &raw_exec.DriverContext{}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: shared.Handshake,
		Plugins: map[string]plugin.Plugin{
			"raw_exec": &shared.RawExecPlugin{Impl: raw_exec.NewRawExecDriver(driverCtx)},
		},

		GRPCServer: plugin.DefaultGRPCServer,
	})
}
