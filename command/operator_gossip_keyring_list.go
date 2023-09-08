// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// OperatorGossipKeyringListCommand is a Command implementation
// that handles removing a gossip encryption key from a keyring
type OperatorGossipKeyringListCommand struct {
	Meta
}

func (c *OperatorGossipKeyringListCommand) Help() string {
	helpText := `
Usage: nomad operator gossip keyring list [options]

  List all gossip keys currently in use within the cluster.

  This command can only be run against server nodes. It returns 0 if all nodes
  reply and there are no errors. If any node fails to reply or reports failure,
  the exit code will be 1.

  If ACLs are enabled, this command requires a token with the 'agent:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *OperatorGossipKeyringListCommand) Synopsis() string {
	return "List gossip encryption keys"
}

func (c *OperatorGossipKeyringListCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *OperatorGossipKeyringListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictAnything
}

func (c *OperatorGossipKeyringListCommand) Name() string { return "operator gossip keyring list" }

func (c *OperatorGossipKeyringListCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("operator-gossip-keyring-list", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	c.Ui = &cli.PrefixedUi{
		OutputPrefix: "",
		InfoPrefix:   "==> ",
		ErrorPrefix:  "",
		Ui:           c.Ui,
	}

	args = flags.Args()
	if len(args) != 0 {
		c.Ui.Error("This command requires no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating nomad cli client: %s", err))
		return 1
	}

	c.Ui.Output("Gathering installed encryption keys...")
	r, err := client.Agent().ListKeys()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error: %s", err))
		return 1
	}
	c.handleKeyResponse(r)
	return 0
}

func (c *OperatorGossipKeyringListCommand) handleKeyResponse(resp *api.KeyringResponse) {
	out := make([]string, len(resp.Keys)+1)
	out[0] = "Key"
	i := 1
	for k := range resp.Keys {
		out[i] = k
		i = i + 1
	}
	c.Ui.Output(formatList(out))
}
