// This package provides a mechanism to build the Docker driver plugin as an
// external binary. The binary has two entry points; the docker driver and the
// docker plugin's logging child binary. An example of using this is `go build
// -o </nomad/plugin/dir/docker`. When Nomad agent is then launched, the
// external docker plugin will be used.
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
			logger := log.New(&log.LoggerOptions{
				Level:      log.Trace,
				JSONFormat: true,
				Name:       docklog.PluginName,
			})

			plugin.Serve(&plugin.ServeConfig{
				HandshakeConfig: base.Handshake,
				Plugins: map[string]plugin.Plugin{
					docklog.PluginName: docklog.NewPlugin(docklog.NewDockerLogger(logger)),
				},
				GRPCServer: plugin.DefaultGRPCServer,
				Logger:     logger,
			})

			return
		}
	}

	// Serve the plugin
	plugins.Serve(factory)
}

// factory returns a new instance of the docker driver plugin
func factory(log log.Logger) interface{} {
	return docker.NewDockerDriver(log)
}
