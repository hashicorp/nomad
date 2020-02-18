package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type CSIPluginStatusCommand struct {
	Meta
	length    int
	evals     bool
	allAllocs bool
	verbose   bool
}

func (c *CSIPluginStatusCommand) Help() string {
	helpText := `
Usage: nomad csi plugin status [options] <id>

  Display status information about a CSI plugin. If no plugin id is given, a
  list of all plugins will be displayed.

General Options:

  ` + generalOptionsUsage() + `

Status Options:

  -short
    Display short output. Used only when a single job is being
    queried, and drops verbose information about allocations.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *CSIPluginStatusCommand) Synopsis() string {
	return "Display status information about a job"
}

func (c *CSIPluginStatusCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-short":   complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *CSIPluginStatusCommand) AutocompleteArgs() complete.Predictor {
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

func (c *CSIPluginStatusCommand) Name() string { return "csi plugin status" }

func (c *CSIPluginStatusCommand) Run(args []string) int {
	var short bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&short, "short", false, "")
	flags.BoolVar(&c.verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no jobs or exactly one.
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error("This command takes either no arguments or one: <plugin id>")
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

	// Invoke list mode if no job ID.
	if len(args) == 0 {
		plugs, _, err := client.CSIPlugins().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying plugins: %s", err))
			return 1
		}

		if len(plugs) == 0 {
			// No output if we have no jobs
			c.Ui.Output("No CSI plugins")
		} else {
			c.Ui.Output(formatCSIPluginList(plugs))
		}
		return 0
	}

	// Try querying the job
	plugID := args[0]

	// Lookup matched a single job
	plug, _, err := client.CSIPlugins().Info(plugID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying plugin: %s", err))
		return 1
	}

	c.Ui.Output(c.formatBasic(plug))

	// Exit early
	if short {
		return 0
	}

	return 0
}

func (v *CSIPluginStatusCommand) formatBasic(plug *api.CSIPlugin) string {
	output := []string{
		fmt.Sprintf("ID|%s", plug.ID),
		fmt.Sprintf("Controllers Healthy|%d", plug.ControllersHealthy),
		fmt.Sprintf("Controllers Expected|%d", len(plug.Controllers)),
		fmt.Sprintf("Nodes Healthy|%d", plug.NodesHealthy),
		fmt.Sprintf("Nodes Expected|%d", len(plug.Nodes)),
	}

	return strings.Join(output, "\n")
}
