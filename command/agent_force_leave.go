package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
)

type AgentForceLeaveCommand struct {
	Ui cli.Ui
}

func (c *AgentForceLeaveCommand) Help() string {
	helpText := `
Usage: nomad agent-force-leave [options] <node>

  Forces an agent to enter the "left" state. This can be used to
  eject nodes which have failed and will not rejoin the cluster.
  Note that if the member is actually still alive, it will
  eventually rejoin the cluster again.

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

func (c *AgentForceLeaveCommand) Synopsis() string {
	return "Forces a member to leave the Nomad cluster"
}

func (c *AgentForceLeaveCommand) Run(args []string) int {
	var httpAddr *string

	flags := flag.NewFlagSet("agent-force-leave", flag.ContinueOnError)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	httpAddr = httpAddrFlag(flags)

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one node
	if len(flags.Args()) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}
	node := flags.Args()[0]

	// Get the HTTP client
	client, err := httpClient(*httpAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed initializing Nomad client: %s", err))
		return 1
	}

	// Call force-leave on the node
	if err := client.Agent().ForceLeave(node); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to force-leave node %s: %s", node, err))
		return 1
	}

	return 0
}
