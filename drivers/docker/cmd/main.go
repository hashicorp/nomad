package main

import (
	"os"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/drivers/docker"
	"github.com/hashicorp/nomad/drivers/docker/docklog"
	"github.com/hashicorp/nomad/plugins"
	"github.com/hashicorp/nomad/plugins/base"
)

func main() {

	if len(os.Args) > 1 {
		// Detect if we are being launched as a docker logging plugin
		switch os.Args[1] {
		case docklog.PluginName:
			plugin.Serve(&plugin.ServeConfig{
				HandshakeConfig: base.Handshake,
				Plugins: map[string]plugin.Plugin{
					docklog.PluginName: docklog.NewPlugin(docklog.NewDockerLogger(log.Default().Named(docklog.PluginName))),
				},
				GRPCServer: plugin.DefaultGRPCServer,
			})

			return
		}
	}

	// Serve the plugin
	plugins.Serve(factory)
}

// factory returns a new instance of the Nvidia GPU plugin
func factory(log log.Logger) interface{} {
	return docker.NewDockerDriver(log)
}
