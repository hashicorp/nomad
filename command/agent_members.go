package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"
)

type AgentMembersCommand struct {
	Ui cli.Ui
}

func (c *AgentMembersCommand) Help() string {
	helpText := `
Usage: nomad agent-members [options]

  Display a list of the known members and their status.

Options:

  -detailed
    Show detailed information about each member. This dumps
    a raw set of tags which shows more information than the
    default output format.

  -help
    Display this message

  -http-addr
    Address of the Nomad API to connect. Can also be specified
    using the environment variable NOMAD_HTTP_ADDR.
    Default = http://127.0.0.1:4646
`
	return strings.TrimSpace(helpText)
}

func (c *AgentMembersCommand) Synopsis() string {
	return "Display a list of known members and their status"
}

func (c *AgentMembersCommand) Run(args []string) int {
	var httpAddr *string
	var detailed bool

	flags := flag.NewFlagSet("agent-members", flag.ContinueOnError)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detailed, "detailed", false, "Show detailed output")
	httpAddr = httpAddrFlag(flags)

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check for extra arguments
	if len(flags.Args()) != 0 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the HTTP client
	client, err := httpClient(*httpAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed initializing Nomad client: %s", err))
		return 1
	}

	// Query the members
	mem, err := client.Agent().Members()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed querying members: %s", err))
		return 1
	}

	// Format the list
	var out []string
	if detailed {
		out = detailedOutput(mem)
	} else {
		out = standardOutput(mem)
	}

	// Dump the list
	c.Ui.Output(columnize.SimpleFormat(out))
	return 0
}

func standardOutput(mem []*api.AgentMember) []string {
	// Format the members list
	members := make([]string, len(mem)+1)
	members[0] = "Name|Addr|Port|Status|Proto|Build|DC|Region"
	for i, member := range mem {
		members[i+1] = fmt.Sprintf("%s|%s|%d|%s|%d|%s|%s|%s",
			member.Name,
			member.Addr,
			member.Port,
			member.Status,
			member.ProtocolCur,
			member.Tags["build"],
			member.Tags["dc"],
			member.Tags["region"])
	}
	return members
}

func detailedOutput(mem []*api.AgentMember) []string {
	// Format the members list
	members := make([]string, len(mem)+1)
	members[0] = "Name|Addr|Port|Tags"
	for i, member := range mem {
		// Format the tags
		tagPairs := make([]string, 0, len(member.Tags))
		for k, v := range member.Tags {
			tagPairs = append(tagPairs, fmt.Sprintf("%s=%s", k, v))
		}
		tags := strings.Join(tagPairs, ",")

		members[i+1] = fmt.Sprintf("%s|%s|%d|%s",
			member.Name,
			member.Addr,
			member.Port,
			tags)
	}
	return members
}
