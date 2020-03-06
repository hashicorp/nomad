package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type VolumeDeregisterCommand struct {
	Meta
}

func (c *VolumeDeregisterCommand) Help() string {
	helpText := `
Usage: nomad volume deregister [options] <id>

  Remove an unused volume from Nomad.

General Options:

  ` + generalOptionsUsage()

	return strings.TrimSpace(helpText)
}

func (c *VolumeDeregisterCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *VolumeDeregisterCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		// When multiple volume types are implemented, this search should merge contexts
		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Volumes, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Volumes]
	})
}

func (c *VolumeDeregisterCommand) Synopsis() string {
	return "Remove a volume"
}

func (c *VolumeDeregisterCommand) Name() string { return "volume deregister" }

func (c *VolumeDeregisterCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing arguments %s", err))
		return 1
	}

	// Check that we get exactly one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <id>")
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

	// Deregister only works on CSI volumes, but could be extended to support other
	// network interfaces or host volumes
	err = client.CSIVolumes().Deregister(volID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deregistering volume: %s", err))
		return 1
	}

	return 0
}
