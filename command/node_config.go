// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type NodeConfigCommand struct {
	Meta
}

func (c *NodeConfigCommand) Help() string {
	helpText := `
Usage: nomad node config [options]

  View or modify a client node's configuration details. This command only works
  on client nodes, and can be used to update the running client configurations
  it supports.

  The arguments behave differently depending on the flags given. See each
  flag's description for its specific requirements.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Client Config Options:

  -servers
    List the known server addresses of the client node. Client nodes do not
    participate in the gossip pool, and instead register with these servers
    periodically over the network.

    If ACLs are enabled, this option requires a token with the 'agent:read'
    capability.

  -update-servers
    Updates the client's server list using the provided arguments. Multiple
    server addresses may be passed using multiple arguments. IMPORTANT: When
    updating the servers list, you must specify ALL of the server nodes you
    wish to configure. The set is updated atomically.

    If ACLs are enabled, this option requires a token with the 'agent:write'
    capability.

    Example:
      $ nomad node config -update-servers foo:4647 bar:4647
`
	return strings.TrimSpace(helpText)
}

func (c *NodeConfigCommand) Synopsis() string {
	return "View or modify client configuration details"
}

func (c *NodeConfigCommand) Name() string { return "node config" }

func (c *NodeConfigCommand) Run(args []string) int {
	var listServers, updateServers bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&listServers, "servers", false, "")
	flags.BoolVar(&updateServers, "update-servers", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	// Check the flags for misuse
	if !listServers && !updateServers {
		c.Ui.Error("The '-servers' or '-update-servers' flag(s) must be set")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if updateServers {
		// Get the server addresses
		if len(args) == 0 {
			c.Ui.Error("If the '-update-servers' flag is set, at least one server argument must be provided")
			c.Ui.Error(commandErrorText(c))
			return 1
		}

		// Set the servers list
		if err := client.Agent().SetServers(args); err != nil {
			c.Ui.Error(fmt.Sprintf("Error updating server list: %s", err))
			return 1
		}
		c.Ui.Output("Updated server list")
		return 0
	}

	if listServers {
		// Query the current server list
		servers, err := client.Agent().Servers()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying server list: %s", err))
			return 1
		}

		// Print the results
		for _, server := range servers {
			c.Ui.Output(server)
		}
		return 0
	}

	// Should not make it this far
	return 1
}

func (c *NodeConfigCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-servers":        complete.PredictNothing,
			"-update-servers": complete.PredictAnything,
		})
}

func (c *NodeConfigCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}
