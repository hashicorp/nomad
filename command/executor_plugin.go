package command

import (
	"strings"

	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/client/driver/plugins"
)

type ExecutorPlugin struct {
	Meta
}

func (e *ExecutorPlugin) Help() string {
	helpText := `
	This is a command used by Nomad internally to launch an executor plugin"
	`
	return strings.TrimSpace(helpText)
}

func (e *ExecutorPlugin) Synopsis() string {
	return "internal - launch an executor plugin"
}

func (e *ExecutorPlugin) Run(args []string) int {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugins.HandshakeConfig,
		Plugins:         plugins.PluginMap,
	})
	return 0
}
