package runner

import (
	"log"
	"os/exec"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/types"
)

// PluginRunner defines the metadata needed to run a plugin with go-plugin.
type PluginRunner struct {
	Name           string
	Type           types.PluginType
	Command        string
	Args           []string
	Builtin        bool
	BuiltinFactory func() (interface{}, error)
}

func (r *PluginRunner) Run(pluginMap map[string]plugin.Plugin,
	hs plugin.HandshakeConfig, reattach *plugin.ReattachConfig, logger *log.Logger) (*plugin.Client, error) {

	cmd := exec.Command(r.Command, r.Args...)
	if reattach != nil {
		// Only one can be set
		cmd = nil
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: hs,
		Plugins:         pluginMap,
		Cmd:             cmd,
		Reattach:        reattach,
		// TODO: Logger: logger,
	})

	return client, nil
}
