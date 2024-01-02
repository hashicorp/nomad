// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	flaghelper "github.com/hashicorp/nomad/helper/flags"
	"github.com/posener/complete"
)

type VolumeDeleteCommand struct {
	Meta
	Secrets string
}

func (c *VolumeDeleteCommand) Help() string {
	helpText := `
Usage: nomad volume delete [options] <vol id>

  Delete a volume from an external storage provider. The volume must still be
  registered with Nomad in order to be deleted. Deleting will fail if the
  volume is still in use by an allocation or in the process of being
  unpublished. If the volume no longer exists, this command will silently
  return without an error.

  When ACLs are enabled, this command requires a token with the
  'csi-write-volume' and 'csi-read-volume' capabilities for the volume's
  namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Delete Options:

  -secret
    Secrets to pass to the plugin to delete the snapshot. Accepts multiple
    flags in the form -secret key=value
`
	return strings.TrimSpace(helpText)
}

func (c *VolumeDeleteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *VolumeDeleteCommand) AutocompleteArgs() complete.Predictor {
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
		matches = append(matches, resp.Matches[contexts.Nodes]...)
		return matches
	})
}

func (c *VolumeDeleteCommand) Synopsis() string {
	return "Delete a volume"
}

func (c *VolumeDeleteCommand) Name() string { return "volume delete" }

func (c *VolumeDeleteCommand) Run(args []string) int {
	var secretsArgs flaghelper.StringFlag
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.Var(&secretsArgs, "secret", "secrets for snapshot, ex. -secret key=value")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing arguments %s", err))
		return 1
	}

	// Check that we get exactly two arguments
	args = flags.Args()
	if l := len(args); l < 1 {
		c.Ui.Error("This command takes at least one argument: <vol id>")
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

	secrets := api.CSISecrets{}
	for _, kv := range secretsArgs {
		if key, value, found := strings.Cut(kv, "="); found {
			secrets[key] = value
		} else {
			c.Ui.Error("Secret must be in the format: -secret key=value")
			return 1
		}
	}

	err = client.CSIVolumes().DeleteOpts(&api.CSIVolumeDeleteRequest{
		ExternalVolumeID: volID,
		Secrets:          secrets,
	}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting volume: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully deleted volume %q!", volID))
	return 0
}
