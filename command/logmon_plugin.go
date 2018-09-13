package command

import (
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/logmon"
)

type LogMonPluginCommand struct {
	Meta
}

func (e *LogMonPluginCommand) Help() string {
	helpText := `
	This is a command used by Nomad internally to launch the logmon process"
	`
	return strings.TrimSpace(helpText)
}

func (e *LogMonPluginCommand) Synopsis() string {
	return "internal - launch a logmon plugin"
}

func (e *LogMonPluginCommand) Run(args []string) int {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: logmon.Handshake,
		Plugins: map[string]plugin.Plugin{
			"logmon": logmon.NewPlugin(logmon.NewLogMon(hclog.Default().Named("logmon.test"))),
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
	return 0
}
