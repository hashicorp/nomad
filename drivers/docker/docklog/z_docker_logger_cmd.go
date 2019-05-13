package docklog

import (
	"os"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == PluginName {
		logger := log.New(&log.LoggerOptions{
			Level:      log.Trace,
			JSONFormat: true,
			Name:       PluginName,
		})

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: base.Handshake,
			Plugins: map[string]plugin.Plugin{
				PluginName: NewPlugin(NewDockerLogger(logger)),
			},
			GRPCServer: plugin.DefaultGRPCServer,
			Logger:     logger,
		})
		os.Exit(0)
	}
}
