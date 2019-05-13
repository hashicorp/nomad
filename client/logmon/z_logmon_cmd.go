package logmon

import (
	"os"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == "logmon" {
		logger := hclog.New(&hclog.LoggerOptions{
			Level:      hclog.Trace,
			JSONFormat: true,
			Name:       "logmon",
		})
		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: base.Handshake,
			Plugins: map[string]plugin.Plugin{
				"logmon": NewPlugin(NewLogMon(logger)),
			},
			GRPCServer: plugin.DefaultGRPCServer,
			Logger:     logger,
		})
		os.Exit(0)
	}
}
