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

type VolumeSnapshotDeleteCommand struct {
	Meta
}

func (c *VolumeSnapshotDeleteCommand) Help() string {
	helpText := `
Usage: nomad volume snapshot delete [options] <plugin id> <snapshot id>

  Delete a snapshot from an external storage provider.

  When ACLs are enabled, this command requires a token with the
  'csi-write-volume' and 'plugin:read' capabilities.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Snapshot Options:

  -secret
    Secrets to pass to the plugin to delete the snapshot. Accepts multiple
    flags in the form -secret key=value

`
	return strings.TrimSpace(helpText)
}

func (c *VolumeSnapshotDeleteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-secret": complete.PredictNothing,
		})
}

func (c *VolumeSnapshotDeleteCommand) AutocompleteArgs() complete.Predictor {
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

func (c *VolumeSnapshotDeleteCommand) Synopsis() string {
	return "Delete a snapshot"
}

func (c *VolumeSnapshotDeleteCommand) Name() string { return "volume snapshot delete" }

func (c *VolumeSnapshotDeleteCommand) Run(args []string) int {
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
	if l := len(args); l < 2 {
		c.Ui.Error("This command takes two arguments: <plugin id> <snapshot id>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	pluginID := args[0]
	snapID := args[1]

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

	err = client.CSIVolumes().DeleteSnapshot(&api.CSISnapshot{
		ID:       snapID,
		PluginID: pluginID,
		Secrets:  secrets,
	}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting volume: %s", err))
		return 1
	}

	return 0
}
