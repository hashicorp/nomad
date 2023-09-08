// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
	"github.com/ryanuber/columnize"
)

type OperatorRaftListCommand struct {
	Meta
}

func (c *OperatorRaftListCommand) Help() string {
	helpText := `
Usage: nomad operator raft list-peers [options]

  Displays the current Raft peer configuration.

  If ACLs are enabled, this command requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

List Peers Options:

  -stale=[true|false]
    The -stale argument defaults to "false" which means the leader provides the
    result. If the cluster is in an outage state without a leader, you may need
    to set -stale to "true" to get the configuration from a non-leader server.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-stale": complete.PredictAnything,
		})
}

func (c *OperatorRaftListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorRaftListCommand) Synopsis() string {
	return "Display the current Raft peer configuration"
}

func (c *OperatorRaftListCommand) Name() string { return "operator raft list-peers" }

func (c *OperatorRaftListCommand) Run(args []string) int {
	var stale bool

	flags := c.Meta.FlagSet("raft", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	flags.BoolVar(&stale, "stale", false, "")
	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Set up a client.
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}
	operator := client.Operator()

	// Fetch the current configuration.
	q := &api.QueryOptions{
		AllowStale: stale,
	}
	reply, err := operator.RaftGetConfiguration(q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to retrieve raft configuration: %v", err))
		return 1
	}

	// Format it as a nice table.
	result := []string{"Node|ID|Address|State|Voter|RaftProtocol"}
	sort.Slice(reply.Servers, func(i, j int) bool {
		return reply.Servers[i].Node < reply.Servers[j].Node
	})

	for _, s := range reply.Servers {
		state := "follower"
		if s.Leader {
			state = "leader"
		}
		result = append(result, fmt.Sprintf("%s|%s|%s|%s|%v|%s",
			s.Node, s.ID, s.Address, state, s.Voter, s.RaftProtocol))
	}
	c.Ui.Output(columnize.SimpleFormat(result))

	return 0
}
