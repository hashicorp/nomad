// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type PluginStatusCommand struct {
	Meta
	length   int
	short    bool
	verbose  bool
	json     bool
	template string
}

func (c *PluginStatusCommand) Help() string {
	helpText := `
Usage nomad plugin status [options] <plugin>

  Display status information about a plugin. If no plugin id is given,
  a list of all plugins will be displayed.

  If ACLs are enabled, this command requires a token with the 'plugin:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Status Options:

  -type <type>
    List only plugins of type <type>.

  -short
    Display short output.

  -verbose
    Display full information.

  -json
    Output the allocation in its JSON format.

  -t
    Format and display allocation using a Go template.
`
	return helpText
}

func (c *PluginStatusCommand) Synopsis() string {
	return "Display status information about a plugin"
}

// predictVolumeType is also used in volume_status
var predictVolumeType = complete.PredictFunc(func(a complete.Args) []string {
	types := []string{"csi"}
	for _, t := range types {
		if strings.Contains(t, a.Last) {
			return []string{t}
		}
	}
	return nil
})

func (c *PluginStatusCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-type":    predictVolumeType,
			"-short":   complete.PredictNothing,
			"-verbose": complete.PredictNothing,
			"-json":    complete.PredictNothing,
			"-t":       complete.PredictAnything,
		})
}

func (c *PluginStatusCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Plugins, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Plugins]
	})
}

func (c *PluginStatusCommand) Name() string { return "plugin status" }

func (c *PluginStatusCommand) Run(args []string) int {
	var typeArg string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&typeArg, "type", "", "")
	flags.BoolVar(&c.short, "short", false, "")
	flags.BoolVar(&c.verbose, "verbose", false, "")
	flags.BoolVar(&c.json, "json", false, "")
	flags.StringVar(&c.template, "t", "", "")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing arguments %s", err))
		return 1
	}

	// Check that we either got no arguments or exactly one.
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error("This command takes either no arguments or one: <plugin>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	typeArg = strings.ToLower(typeArg)

	// Check that the plugin type flag is supported. Empty implies we are
	// querying all plugins, otherwise we currently only support "csi".
	switch typeArg {
	case "", "csi":
	default:
		c.Ui.Error(fmt.Sprintf("Unsupported plugin type: %s", typeArg))
		return 1
	}

	// Truncate the id unless full length is requested
	c.length = shortId
	if c.verbose {
		c.length = fullId
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	id := ""
	if len(args) == 1 {
		id = args[0]
	}

	code := c.csiStatus(client, id)
	if code != 0 {
		return code
	}

	// Extend this section with other plugin implementations

	return 0
}
