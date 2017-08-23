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
can be used to remove the failed server so that it is no longer affects the Raft
quorum. If the server still shows in the output of the "nomad server-members"
command, it is preferable to clean up by simply running "nomad
server-force-leave" instead of this command.

General Options:

  ` + generalOptionsUsage() + `

Remove Peer Options:

  -peer-address="IP:port"
    Remove a Nomad server with given address from the Raft configuration.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftRemoveCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-peer-address": complete.PredictAnything,
		})
}

func (c *OperatorRaftRemoveCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorRaftRemoveCommand) Synopsis() string {
	return "Remove a Nomad server from the Raft configuration"
}

func (c *OperatorRaftRemoveCommand) Run(args []string) int {
	var peerAddress string

	flags := c.Meta.FlagSet("raft", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	flags.StringVar(&peerAddress, "peer-address", "", "")
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

	// TODO (alexdadgar) Once we expose IDs, add support for removing
	// by ID, add support for that.
	if len(peerAddress) == 0 {
		c.Ui.Error(fmt.Sprintf("an address is required for the peer to remove"))
		return 1
	}

	// Try to kick the peer.
	w := &api.WriteOptions{}
	if err := operator.RaftRemovePeerByAddress(peerAddress, w); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to remove raft peer: %v", err))
		return 1
	}
	c.Ui.Output(fmt.Sprintf("Removed peer with address %q", peerAddress))

	return 0
}
