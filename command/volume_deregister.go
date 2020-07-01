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

  ` + generalOptionsUsage() + `

Volume Deregister Options:

  -force
    Force deregistration of the volume and immediately drop claims for
    terminal allocations. Returns an error if the volume has running
    allocations. This does not detach the volume from client nodes.
`
	return strings.TrimSpace(helpText)
}

func (c *VolumeDeregisterCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-force": complete.PredictNothing,
		})
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
	var force bool
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&force, "force", false, "Force deregister and drop claims")

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

	// Confirm the -force flag
	if force {
		question := fmt.Sprintf("Are you sure you want to force deregister volume %q? [y/N]", volID)
		answer, err := c.Ui.Ask(question)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse answer: %v", err))
			return 1
		}

		if answer == "" || strings.ToLower(answer)[0] == 'n' {
			// No case
			c.Ui.Output("Cancelling volume deregister")
			return 0
		} else if strings.ToLower(answer)[0] == 'y' && len(answer) > 1 {
			// Non exact match yes
			c.Ui.Output("For confirmation, an exact ‘y’ is required.")
			return 0
		} else if answer != "y" {
			c.Ui.Output("No confirmation detected. For confirmation, an exact 'y' is required.")
			return 1
		}
	}

	// Deregister only works on CSI volumes, but could be extended to support other
	// network interfaces or host volumes
	err = client.CSIVolumes().Deregister(volID, force, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deregistering volume: %s", err))
		return 1
	}

	return 0
}
