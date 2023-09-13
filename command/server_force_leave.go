// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type ServerForceLeaveCommand struct {
	Meta
}

func (c *ServerForceLeaveCommand) Help() string {
	helpText := `
Usage: nomad server force-leave [options] <node>

  Forces an server to enter the "left" state. This can be used to
  eject nodes which have failed and will not rejoin the cluster.
  Note that if the member is actually still alive, it will
  eventually rejoin the cluster again. The failed or left server will
  be garbage collected after 24h.

  If ACLs are enabled, this option requires a token with the 'agent:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `
  
Server Force-Leave Options:

  -prune
    Removes failed or left server from the Serf member list immediately.
	This will not work with alive server.
`
	return strings.TrimSpace(helpText)
}

func (c *ServerForceLeaveCommand) Synopsis() string {
	return "Force a server into the 'left' state"
}

func (c *ServerForceLeaveCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-prune": complete.PredictNothing,
		})
}

func (c *ServerForceLeaveCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ServerForceLeaveCommand) Name() string { return "server force-leave" }

func (c *ServerForceLeaveCommand) Run(args []string) int {
	var prune bool
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&prune, "prune", false, "Remove server completely from list of members")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one node
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <node>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	node := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Call force-leave on the node
	forceLeaveOpts := api.ForceLeaveOpts{
		Prune: prune,
	}
	if err := client.Agent().ForceLeaveWithOptions(node, forceLeaveOpts); err != nil {
		c.Ui.Error(fmt.Sprintf("Error force-leaving server %s: %s", node, err))
		return 1
	}

	return 0
}
