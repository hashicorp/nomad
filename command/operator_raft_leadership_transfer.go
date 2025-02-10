// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type OperatorRaftTransferLeadershipCommand struct {
	Meta
}

func (c *OperatorRaftTransferLeadershipCommand) Help() string {
	helpText := `
Usage: nomad operator raft transfer-leadership [options]

  Transfer leadership to the Nomad server with given -peer-address or
  -peer-id in the Raft configuration. All server nodes in the cluster
  must be running at least Raft protocol v3 in order to use this command.

  There are cases where you might desire transferring leadership from one
  cluster member to another, for example, during a rolling upgrade. This
  command allows you to designate a new server to be cluster leader.

  Note: This command requires a currently established leader to function.

  If ACLs are enabled, this command requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Transfer Leadership Options:

  -peer-address="IP:port"
    Transfer leadership to the Nomad server with given Raft address.

  -peer-id="id"
    Transfer leadership to the Nomad server with given Raft ID.
`

	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftTransferLeadershipCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-peer-address": complete.PredictAnything,
			"-peer-id":      complete.PredictAnything,
		})
}

func (c *OperatorRaftTransferLeadershipCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorRaftTransferLeadershipCommand) Synopsis() string {
	return "Transfer leadership to a specified Nomad server"
}

func (c *OperatorRaftTransferLeadershipCommand) Name() string {
	return "operator raft transfer-leadership"
}

func (c *OperatorRaftTransferLeadershipCommand) Run(args []string) int {
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

	if err := raftTransferLeadership(peerAddress, peerID, operator); err != nil {
		c.Ui.Error(fmt.Sprintf("Error transferring leadership to peer: %v", err))
		return 1
	}
	if peerAddress != "" {
		c.Ui.Output(fmt.Sprintf("Transferred leadership to peer with address %q", peerAddress))
	} else {
		c.Ui.Output(fmt.Sprintf("Transferred leadership to peer with id %q", peerID))
	}

	return 0
}

func raftTransferLeadership(address, id string, operator *api.Operator) error {
	if len(address) == 0 && len(id) == 0 {
		return fmt.Errorf("an address or id is required for the destination peer")
	}
	if len(address) > 0 && len(id) > 0 {
		return fmt.Errorf("cannot give both an address and id")
	}

	// Try to perform the leadership transfer.
	if len(address) > 0 {
		if err := operator.RaftTransferLeadershipByAddress(address, nil); err != nil {
			return err
		}
	} else {
		if err := operator.RaftTransferLeadershipByID(id, nil); err != nil {
			return err
		}
	}

	return nil
}
