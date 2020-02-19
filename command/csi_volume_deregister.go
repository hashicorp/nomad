package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type CSIVolumeDeregisterCommand struct {
	Meta
}

func (c *CSIVolumeDeregisterCommand) Help() string {
	helpText := `
Usage: nomad csi volume deregister [options] <input>

  Deregister is used to create or update a CSI volume specification. The volume
  must exist on the remote storage provider before it can be used by a task.
  The specification file will be read from stdin by specifying "-", otherwise
  a path to the file is expected.

General Options:

  ` + generalOptionsUsage() + `

Deregister Options:
`

	return strings.TrimSpace(helpText)
}

func (c *CSIVolumeDeregisterCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
		})
}

func (c *CSIVolumeDeregisterCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFiles("*")
}

func (c *CSIVolumeDeregisterCommand) Synopsis() string {
	return "Create or update a CSI volume specification"
}

func (c *CSIVolumeDeregisterCommand) Name() string { return "csi volume deregister" }

func (c *CSIVolumeDeregisterCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we get exactly one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <volume id>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	volID := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	err = client.CSIVolumes().Deregister(volID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deregistering volume: %s", err))
		return 1
	}

	return 0
}
