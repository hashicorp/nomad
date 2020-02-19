package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type CSIVolumeListCommand struct {
	Meta
}

func (c *CSIVolumeListCommand) Help() string {
	helpText := `
Usage: nomad csi volume list [options]

  Display the list of registered volumes.

General Options:

  ` + generalOptionsUsage() + `

List Options:

  -json
   Output the list in a JSON format.

  -t <template>
   Format and display the volumes using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *CSIVolumeListCommand) Synopsis() string {
	return "Display the list of registered volumes"
}

func (c *CSIVolumeListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *CSIVolumeListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *CSIVolumeListCommand) Name() string { return "csi volume list" }

func (c *CSIVolumeListCommand) Run(args []string) int {
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

	vols, _, err := client.CSIVolumes().List(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying jobs: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, vols)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatCSIVolumeList(vols))
	return 0
}

func formatCSIVolumeList(vols []*api.CSIVolumeListStub) string {
	if len(vols) == 0 {
		return "No volumes found"
	}

	// Sort the output by volume id
	sort.Slice(vols, func(i, j int) bool { return vols[i].ID < vols[j].ID })

	rows := make([]string, len(vols)+1)
	rows[0] = "ID|Plugin ID|Healthy|Access Mode"
	for i, v := range vols {
		rows[i+1] = fmt.Sprintf("%s|%s|%t|%s",
			v.ID,
			v.PluginID,
			v.Healthy,
			v.AccessMode,
		)
	}
	return formatList(rows)
}
