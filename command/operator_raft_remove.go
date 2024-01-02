// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type OperatorRaftRemoveCommand struct {
	Meta
}

func (c *OperatorRaftRemoveCommand) Help() string {
	helpText := `
Usage: nomad operator raft remove-peer [options]

  Remove the Nomad server with given -peer-address from the Raft configuration.

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

  -peer-address="IP:port"
	Remove a Nomad server with given address from the Raft configuration.

  -peer-id="id"
	Remove a Nomad server with the given ID from the Raft configuration.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftRemoveCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-peer-address": complete.PredictAnything,
			"-peer-id":      complete.PredictAnything,
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
	var peerAddress string
	var peerID string

	flags := c.Meta.FlagSet("raft", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	flags.StringVar(&peerAddress, "peer-address", "", "")
	flags.StringVar(&peerID, "peer-id", "", "")
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

	if err := raftRemovePeers(peerAddress, peerID, operator); err != nil {
		c.Ui.Error(fmt.Sprintf("Error removing peer: %v", err))
		return 1
	}
	if peerAddress != "" {
		c.Ui.Output(fmt.Sprintf("Removed peer with address %q", peerAddress))
	} else {
		c.Ui.Output(fmt.Sprintf("Removed peer with id %q", peerID))
	}

	return 0
}

func raftRemovePeers(address, id string, operator *api.Operator) error {
	if len(address) == 0 && len(id) == 0 {
		return fmt.Errorf("an address or id is required for the peer to remove")
	}
	if len(address) > 0 && len(id) > 0 {
		return fmt.Errorf("cannot give both an address and id")
	}

	// Try to kick the peer.
	if len(address) > 0 {
		if err := operator.RaftRemovePeerByAddress(address, nil); err != nil {
			return err
		}
	} else {
		if err := operator.RaftRemovePeerByID(id, nil); err != nil {
			return err
		}
	}

	return nil
}
