package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type CSIPluginListCommand struct {
	Meta
}

func (c *CSIPluginListCommand) Help() string {
	helpText := `
Usage: nomad csi plugin list [options]

  Display the list of registered plugins.

General Options:

  ` + generalOptionsUsage() + `

List Options:

  -json
   Output the list in a JSON format.

  -t <template>
   Format and display the plugins using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *CSIPluginListCommand) Synopsis() string {
	return "Display the list of registered plugins"
}

func (c *CSIPluginListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *CSIPluginListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *CSIPluginListCommand) Name() string { return "csi plugin list" }

func (c *CSIPluginListCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no jobs or exactly one.
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}
	plugs, _, err := client.CSIPlugins().List(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying jobs: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, plugs)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatCSIPluginList(plugs))
	return 0
}

func formatCSIPluginList(plugs []*api.CSIPluginListStub) string {
	if len(plugs) == 0 {
		return "No plugins found"
	}

	// Sort the output by quota name
	sort.Slice(plugs, func(i, j int) bool { return plugs[i].ID < plugs[j].ID })

	rows := make([]string, len(plugs)+1)
	rows[0] = "ID|Controllers Healthy|Controllers Expected|Nodes Healthy|Nodes Expected"
	for i, p := range plugs {
		rows[i+1] = fmt.Sprintf("%s|%d|%d|%d|%d",
			p.ID,
			p.ControllersHealthy,
			p.ControllersExpected,
			p.NodesHealthy,
			p.NodesExpected,
		)
	}
	return formatList(rows)
}
