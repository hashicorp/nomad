// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
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

  When ACLs are enabled, this command requires a token with the
  'csi-write-volume' and 'csi-read-volume' capabilities for the volume's
  namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

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
		matches = append(matches, resp.Matches[contexts.Nodes]...)
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

	nodeID = sanitizeUUIDPrefix(nodeID)
	nodes, _, err := client.Nodes().PrefixList(nodeID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error detaching volume: %s", err))
		return 1
	}

	if len(nodes) > 1 {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple nodes\n\n%s",
			formatNodeStubList(nodes, true)))
		return 1
	}

	if len(nodes) == 1 {
		nodeID = nodes[0].ID
	}

	// If the Nodes.PrefixList doesn't return a node, the node may have been
	// GC'd. The unpublish workflow gracefully handles this case so that we
	// can free the claim. Make a best effort to find a node ID among the
	// volume's claimed allocations, otherwise just use the node ID we've been
	// given.
	if len(nodes) == 0 {

		// Prefix search for the volume
		vols, _, err := client.CSIVolumes().List(&api.QueryOptions{Prefix: volID})
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying volumes: %s", err))
			return 1
		}
		if len(vols) == 0 {
			c.Ui.Error(fmt.Sprintf("No volumes(s) with prefix or ID %q found", volID))
			return 1
		}
		if len(vols) > 1 {
			if (volID != vols[0].ID) || (c.allNamespaces() && vols[0].ID == vols[1].ID) {
				sort.Slice(vols, func(i, j int) bool { return vols[i].ID < vols[j].ID })
				out, err := csiFormatSortedVolumes(vols)
				if err != nil {
					c.Ui.Error(fmt.Sprintf("Error formatting: %s", err))
					return 1
				}
				c.Ui.Error(fmt.Sprintf("Prefix matched multiple volumes\n\n%s", out))
				return 1
			}
		}
		volID = vols[0].ID

		vol, _, err := client.CSIVolumes().Info(volID, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying volume: %s", err))
			return 1
		}
		nodeIDs := []string{}
		for _, alloc := range vol.Allocations {
			if strings.HasPrefix(alloc.NodeID, nodeID) {
				nodeIDs = append(nodeIDs, alloc.NodeID)
			}
		}
		if len(nodeIDs) > 1 {
			c.Ui.Error(fmt.Sprintf("Prefix matched multiple node IDs\n\n%s",
				formatList(nodeIDs)))
		}
		if len(nodeIDs) == 1 {
			nodeID = nodeIDs[0]
		}
	}

	err = client.CSIVolumes().Detach(volID, nodeID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error detaching volume: %s", err))
		return 1
	}

	return 0
}
