package main

import (
	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/plugins/drivers/raw_exec_driver/plugin-go-grpc/raw_exec"
	"github.com/hashicorp/nomad/plugins/drivers/raw_exec_driver/shared"
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
