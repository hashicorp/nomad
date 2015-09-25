package command

import (
	"fmt"
	"strings"
)

type ClientServersCommand struct {
	Meta
}

func (c *ClientServersCommand) Help() string {
	helpText := `
Usage: nomad client-servers [options] [<address>...]

  Query or modify the list of servers on a client node. Client
  nodes do not participate in the gossip protocol, and register
  with server nodes periodically over the network. This command
  can be used to list or modify the set of servers Nomad speaks
  to during this process.

Examples

    $ nomad client-servers -update foo:4647 bar:4647 baz:4647

General Options:

  ` + generalOptionsUsage() + `

Client Servers Options:

  -update
    Updates the client's server list using the provided
    arguments. Multiple server addresses may be passed using
    multiple arguments. IMPORTANT: When updating the servers
    list, you must specify ALL of the server nodes you wish
    to configure. The set is updated atomically.
`
	return strings.TrimSpace(helpText)
}

func (c *ClientServersCommand) Synopsis() string {
	return "Query or modify the list of client servers"
}

func (c *ClientServersCommand) Run(args []string) int {
	var update bool

	flags := c.Meta.FlagSet("client-servers", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&update, "update", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check the flags for misuse
	args = flags.Args()
	if len(args) > 0 && !update {
		c.Ui.Error(c.Help())
		return 1
	}
	if len(args) == 0 && update {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if update {
		// Set the servers list
		if err := client.Agent().SetServers(args); err != nil {
			c.Ui.Error(fmt.Sprintf("Error updating server list: %s", err))
			return 1
		}
		return 0
	}

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
