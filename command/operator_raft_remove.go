// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type OperatorRaftRemoveCommand struct {
	Meta
}

func (c *OperatorRaftRemoveCommand) Help() string {
	helpText := `
Usage: nomad operator raft remove-peer [options]

  Remove the Nomad server with given -peer-id from the Raft configuration.

  There are rare cases where a peer may be left behind in the Raft quorum even
  though the server is no longer present and known to the cluster. This command
  can be used to remove the failed server so that it is no longer affects the
  Raft quorum. If the server still shows in the output of the "nomad
  server-members" command, it is preferable to clean up by simply running "nomad
  server-force-leave" instead of this command.

  If ACLs are enabled, this command requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Remove Peer Options:

  -peer-id="id"
	Remove a Nomad server with the given ID from the Raft configuration.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftRemoveCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-peer-id": complete.PredictAnything,
		})
}

func (c *OperatorRaftRemoveCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorRaftRemoveCommand) Synopsis() string {
	return "Remove a Nomad server from the Raft configuration"
}

func (c *OperatorRaftRemoveCommand) Name() string { return "operator raft remove-peer" }

func (c *OperatorRaftRemoveCommand) Run(args []string) int {
	var peerID string

	flags := c.Meta.FlagSet("raft", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	flags.StringVar(&peerID, "peer-id", "", "")
	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}
	if peerID == "" {
		c.Ui.Error("Missing peer id required")
		return 1
	}

	// Set up a client.
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}
	operator := client.Operator()

	if err := operator.RaftRemovePeerByID(peerID, nil); err != nil {
		c.Ui.Error(fmt.Sprintf("Error removing peer: %v", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Removed peer with id %q", peerID))
	return 0
}
