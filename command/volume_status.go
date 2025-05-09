// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type VolumeStatusCommand struct {
	Meta
	length   int
	short    bool
	verbose  bool
	json     bool
	template string
}

func (c *VolumeStatusCommand) Help() string {
	helpText := `
Usage: nomad volume status [options] <id>

  Display status information about a CSI volume. If no volume id is given, a
  list of all volumes will be displayed.

  When ACLs are enabled, this command requires a token with the
  'csi-read-volume' and 'csi-list-volumes' capability for the volume's
  namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Status Options:

  -type <type>
    List only volumes of type <type> (one of "host" or "csi"). If omitted, the
    command will query for both dynamic host volumes and CSI volumes.

  -short
    Display short output. Used only when a single volume is being
    queried, and drops verbose information about allocations.

  -verbose
    Display full volumes information.

  -json
    Output the volumes in JSON format.

  -t
    Format and display volumes using a Go template.

  -node-pool <pool>
    Filter results by node pool, when no volume ID is provided and -type=host.

  -node <node ID>
    Filter results by node ID, when no volume ID is provided and -type=host.
`
	return strings.TrimSpace(helpText)
}

func (c *VolumeStatusCommand) Synopsis() string {
	return "Display status information about a volume"
}

func (c *VolumeStatusCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-type":      complete.PredictSet("csi", "host"),
			"-short":     complete.PredictNothing,
			"-verbose":   complete.PredictNothing,
			"-json":      complete.PredictNothing,
			"-t":         complete.PredictAnything,
			"-node":      nodePredictor(c.Client, nil),
			"-node-pool": nodePoolPredictor(c.Client, nil),
		})
}

func (c *VolumeStatusCommand) AutocompleteArgs() complete.Predictor {
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

func (c *VolumeStatusCommand) Name() string { return "volume status" }

func (c *VolumeStatusCommand) Run(args []string) int {
	var typeArg, nodeID, nodePool string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&typeArg, "type", "", "")
	flags.BoolVar(&c.short, "short", false, "")
	flags.BoolVar(&c.verbose, "verbose", false, "")
	flags.BoolVar(&c.json, "json", false, "")
	flags.StringVar(&c.template, "t", "", "")
	flags.StringVar(&nodeID, "node", "", "")
	flags.StringVar(&nodePool, "node-pool", "", "")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing arguments %s", err))
		return 1
	}

	// Check that we either got no arguments or exactly one
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error("This command takes either no arguments or one: <id>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Truncate alloc and node IDs unless full length is requested
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

	id := ""
	if len(args) == 1 {
		id = args[0]
	}

	opts := formatOpts{
		verbose:  c.verbose,
		short:    c.short,
		length:   c.length,
		json:     c.json,
		template: c.template,
	}

	switch typeArg {
	case "csi":
		if nodeID != "" || nodePool != "" {
			c.Ui.Error("-node and -node-pool can only be used with -type host")
			return 1
		}
		if err := c.csiVolumeStatus(client, id, opts); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	case "host":
		if err := c.hostVolumeStatus(client, id, nodeID, nodePool, opts); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	case "":
		if id == "" {
			// for list, we want to show both
			dhvErr := c.hostVolumeList(client, nodeID, nodePool, opts)
			if dhvErr != nil {
				c.Ui.Error(dhvErr.Error())
			}
			c.Ui.Output("")
			csiErr := c.csiVolumesList(client, opts)
			if csiErr != nil {
				c.Ui.Error(csiErr.Error())
			}
			if dhvErr == nil && csiErr == nil {
				return 0
			}
			return 1
		} else {
			// for read, we only want to show whichever has results
			hostErr := c.hostVolumeStatus(client, id, nodeID, nodePool, opts)
			if hostErr != nil {
				if !errors.Is(hostErr, hostVolumeListError) {
					c.Ui.Error(hostErr.Error())
					return 1 // we found a host volume but had some other error
				}
				csiErr := c.csiVolumeStatus(client, id, opts)
				if csiErr != nil {
					c.Ui.Error(hostErr.Error())
					c.Ui.Error(csiErr.Error())
					return 1
				}
			}
		}
	default:
		c.Ui.Error(fmt.Sprintf("No such volume type %q", typeArg))
		return 1
	}

	return 0
}
