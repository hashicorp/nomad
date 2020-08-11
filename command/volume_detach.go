package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type VolumeDetachCommand struct {
	Meta
}

func (c *VolumeDetachCommand) Help() string {
	helpText := `
Usage: nomad volume detach [options] <vol id> <node id>

  Detach a volume from a Nomad client.

General Options:

  ` + generalOptionsUsage() + `

`
	return strings.TrimSpace(helpText)
}

func (c *VolumeDetachCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *VolumeDetachCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Volumes, nil)
		if err != nil {
			return []string{}
		}
		matches := resp.Matches[contexts.Volumes]

		resp, _, err = client.Search().PrefixSearch(a.Last, contexts.Nodes, nil)
		if err != nil {
			return []string{}
		}
		for _, match := range resp.Matches[contexts.Nodes] {
			matches = append(matches, match)
		}
		return matches
	})
}

func (c *VolumeDetachCommand) Synopsis() string {
	return "Detach a volume"
}

func (c *VolumeDetachCommand) Name() string { return "volume detach" }

func (c *VolumeDetachCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing arguments %s", err))
		return 1
	}

	// Check that we get exactly two arguments
	args = flags.Args()
	if l := len(args); l != 2 {
		c.Ui.Error("This command takes two arguments: <vol id> <node id>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	volID := args[0]
	nodeID := args[1]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	err = client.CSIVolumes().Detach(volID, nodeID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error detaching volume: %s", err))
		return 1
	}

	return 0
}
