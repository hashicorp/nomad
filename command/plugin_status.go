package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type PluginStatusCommand struct {
	Meta
	length  int
	verbose bool
}

func (c *PluginStatusCommand) Help() string {
	helpText := `
Usage nomad plugin status [options] [plugin]

    Display status information about a plugin. If no plugin id is given,
    a list of all plugins will be displayed.

General Options:

  ` + generalOptionsUsage() + `

Status Options:

  -type <type>
    List only plugins of type <type>.

  -short
    Display short output.

  -verbose
    Display full information.
`
	return helpText
}

func (c *PluginStatusCommand) Synopsis() string {
	return "Display status information about a plugin, or a list of plugins"
}

func (c *PluginStatusCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-type":    complete.PredictAnything, // FIXME predict type
			"-short":   complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *PluginStatusCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.CSIPlugins, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.CSIPlugins]
	})
}

func (c *PluginStatusCommand) Name() string { return "plugin status" }

func (c *PluginStatusCommand) Run(args []string) int {
	var short bool
	var typeArg string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&typeArg, "type", "", "")
	flags.BoolVar(&short, "short", false, "")
	flags.BoolVar(&c.verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	typeArg = strings.ToLower(typeArg)

	// Check that we either got no arguments or exactly one.
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error("This command takes either no arguments or one: <plugin>")
		c.Ui.Error(commandErrorText(c))
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

	c.Ui.Output(c.Colorize().Color("\n[bold]Container Storage Interface[reset]"))
	code := c.csiStatus(client, short, id)
	if code != 0 {
		return code
	}

	if typeArg == "csi" {
		return 0
	}

	// Extend this section with other plugin implementations

	return 0
}
