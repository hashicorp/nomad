// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/hashicorp/nomad/helper"
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
    flags in the form -secret key=value. Only available for CSI volumes.

  -type <type>
    Type of volume to delete. Must be one of "csi" or "host". Defaults to "csi".
`
	return strings.TrimSpace(helpText)
}

func (c *VolumeDeleteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-type":   complete.PredictSet("csi", "host"),
			"-secret": complete.PredictNothing,
		})
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

		resp, _, err = client.Search().PrefixSearch(a.Last, contexts.HostVolumes, nil)
		if err != nil {
			return []string{}
		}
		matches = append(matches, resp.Matches[contexts.HostVolumes]...)
		return matches
	})
}

func (c *VolumeDeleteCommand) Synopsis() string {
	return "Delete a volume"
}

func (c *VolumeDeleteCommand) Name() string { return "volume delete" }

func (c *VolumeDeleteCommand) Run(args []string) int {
	var secretsArgs flaghelper.StringFlag
	var typeArg string
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.Var(&secretsArgs, "secret", "secrets for snapshot, ex. -secret key=value")
	flags.StringVar(&typeArg, "type", "csi", "type of volume (csi or host)")

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

	switch typeArg {
	case "csi":
		return c.deleteCSIVolume(client, volID, secretsArgs)
	case "host":
		return c.deleteHostVolume(client, volID)
	default:
		c.Ui.Error(fmt.Sprintf("No such volume type %q", typeArg))
		return 1
	}
}

func (c *VolumeDeleteCommand) deleteCSIVolume(client *api.Client, volID string, secretsArgs flaghelper.StringFlag) int {

	secrets := api.CSISecrets{}
	for _, kv := range secretsArgs {
		if key, value, found := strings.Cut(kv, "="); found {
			secrets[key] = value
		} else {
			c.Ui.Error("Secret must be in the format: -secret key=value")
			return 1
		}
	}

	// get a CSI volume that matches the given prefix or a list of all matches
	// if an exact match is not found.
	stub, possible, err := getByPrefix[api.CSIVolumeListStub]("volumes", client.CSIVolumes().List,
		func(vol *api.CSIVolumeListStub, prefix string) bool { return vol.ID == prefix },
		&api.QueryOptions{
			Prefix:    volID,
			Namespace: c.namespace,
		})
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Could not find existing volume to delete: %s", err))
		return 1
	}
	if len(possible) > 0 {
		out, err := csiFormatVolumes(possible, false, "")
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error formatting: %s", err))
			return 1
		}
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple volumes\n\n%s", out))
		return 1
	}
	volID = stub.ID
	c.namespace = stub.Namespace

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

func (c *VolumeDeleteCommand) deleteHostVolume(client *api.Client, volID string) int {

	if !helper.IsUUID(volID) {
		stub, possible, err := getHostVolumeByPrefix(client, volID, c.namespace)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Could not find existing volume to delete: %s", err))
			return 1
		}
		if len(possible) > 0 {
			out, err := formatHostVolumes(possible, formatOpts{short: true})
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error formatting: %s", err))
				return 1
			}
			c.Ui.Error(fmt.Sprintf("Prefix matched multiple volumes\n\n%s", out))
			return 1
		}
		volID = stub.ID
		c.namespace = stub.Namespace
	}

	_, err := client.HostVolumes().Delete(&api.HostVolumeDeleteRequest{ID: volID}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting volume: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully deleted volume %q!", volID))
	return 0
}
