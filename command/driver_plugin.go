package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-plugin"
	driver "github.com/hashicorp/nomad/client/drivernew/plugin"
	"github.com/hashicorp/nomad/plugins/builtin"
	"github.com/hashicorp/nomad/plugins/types"
)

type DriverPluginCommand struct {
	Meta
}

func (e *DriverPluginCommand) Help() string {
	helpText := `
	Internal command to launch a built-in driver
	`
	return strings.TrimSpace(helpText)
}

func (e *DriverPluginCommand) Synopsis() string {
	return "internal - launch an built-in driver"
}

// Run takes as its only argument the driver name to launch as a plugin
func (e *DriverPluginCommand) Run(args []string) int {
	if len(args) != 1 {
		e.Ui.Error("require driver name")
		return 1
	}

	// Get the driver implementation
	name := args[0]
	runner, ok := builtin.Plugins[types.Driver][name]
	if !ok {
		e.Ui.Error(fmt.Sprintf("unknown driver %q", name))
		return 1
	}

	impl, err := runner.BuiltinFactory()
	if err != nil {
		e.Ui.Error(fmt.Sprintf("failed creating driver %q: %s", name, err))
		return 1
	}

	d, ok := impl.(driver.Driver)
	if !ok {
		e.Ui.Error(fmt.Sprintf("plugin %q doesn't meet driver interface", name))
		return 1
	}

	pluginMap := map[string]plugin.Plugin{
		"driver": driver.NewDriverPlugin(d),
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: driver.HandshakeConfig,
		Plugins:         pluginMap,
	})

	return 0
}
