package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
)

type AgentJoinCommand struct {
	Ui cli.Ui
}

func (c *AgentJoinCommand) Help() string {
	helpText := `
Usage: nomad agent-join [options] <addr> [<addr>...]

  Joins the local server to one or more Nomad servers. Joining is
  only required for server nodes, and only needs to succeed
  against one or more of the provided addresses. Once joined, the
  gossip layer will handle discovery of the other server nodes in
  the cluster.

Options:

  -help
    Display this message

  -http-addr
    Address of the Nomad API to connect. Can also be specified
    using the environment variable NOMAD_HTTP_ADDR.
    Default = http://127.0.0.1:4646
`
	return strings.TrimSpace(helpText)
}

func (c *AgentJoinCommand) Synopsis() string {
	return "Joins server nodes together"
}

func (c *AgentJoinCommand) Run(args []string) int {
	var httpAddr *string

	flags := flag.NewFlagSet("agent-join", flag.ContinueOnError)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	httpAddr = httpAddrFlag(flags)

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got at least one node
	if len(flags.Args()) < 1 {
		c.Ui.Error(c.Help())
		return 1
	}
	nodes := flags.Args()

	// Get the HTTP client
	client, err := httpClient(*httpAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed initializing Nomad client: %s", err))
		return 1
	}

	// Attempt the join
	n, err := client.Agent().Join(nodes...)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to join: %s", err))
		return 1
	}

	// Success
	c.Ui.Output(fmt.Sprintf("Joined %d nodes successfully", n))
	return 0
}
