package command

import (
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/drivers/docker/docklog"
	"github.com/hashicorp/nomad/plugins/base"
)

type DockerLoggerPluginCommand struct {
	Meta
}

func (e *DockerLoggerPluginCommand) Help() string {
	helpText := `
	This is a command used by Nomad internally to launch the docker logger process"
	`
	return strings.TrimSpace(helpText)
}

func (e *DockerLoggerPluginCommand) Synopsis() string {
	return "internal - launch a docker logger plugin"
}

func (e *DockerLoggerPluginCommand) Run(args []string) int {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: base.Handshake,
		Plugins: map[string]plugin.Plugin{
			docklog.PluginName: docklog.NewPlugin(docklog.NewDockerLogger(hclog.Default().Named(docklog.PluginName))),
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
	return 0
}
