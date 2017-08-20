package builtin

import (
	"github.com/hashicorp/nomad/client/drivernew/exec"
	"github.com/hashicorp/nomad/plugins/runner"
	"github.com/hashicorp/nomad/plugins/types"
)

const (
	// driverPluginCommand is the command used on Nomad to run the driver plugin
	driverPluginCommand = "driver_plugin"
)

var (
	// Plugins are the set of built in plugin
	Plugins = map[types.PluginType]map[string]*runner.PluginRunner{
		types.Driver: map[string]*runner.PluginRunner{
			exec.Name: &runner.PluginRunner{
				Name:           exec.Name,
				Type:           types.Driver,
				Args:           []string{driverPluginCommand, exec.Name},
				Builtin:        true,
				BuiltinFactory: exec.New,
			},
		},
	}
)

// GetRunner returns the builtin runner. The executable path is not set on the
// returned runner.
func GetRunner(t types.PluginType, name string) *runner.PluginRunner {
	runner, ok := Plugins[t][name]
	if !ok {
		return nil
	}

	return runner
}
