package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
)

type AgentInfoCommand struct {
	Ui cli.Ui
}

func (c *AgentInfoCommand) Help() string {
	helpText := `
Usage: nomad agent-info [options]

  Displays status information about the local agent.

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

func (c *AgentInfoCommand) Synopsis() string {
	return "Displays local agent information and status"
}

func (c *AgentInfoCommand) Run(args []string) int {
	var httpAddr *string

	flags := flag.NewFlagSet("agent-info", flag.ContinueOnError)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	httpAddr = httpAddrFlag(flags)

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no jobs or exactly one.
	if len(flags.Args()) > 0 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the HTTP client
	client, err := httpClient(*httpAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed initializing Nomad client: %s", err))
		return 1
	}

	// Query the agent info
	info, err := client.Agent().Self()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed querying agent info: %s", err))
		return 1
	}

	var stats map[string]interface{}
	stats, _ = info["stats"]

	for section, data := range stats {
		c.Ui.Output(section)
		d, _ := data.(map[string]interface{})
		for k, v := range d {
			c.Ui.Output(fmt.Sprintf("  %s = %v", k, v))
		}
	}

	return 0
}
