// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// OperatorGossipKeyringRemoveCommand is a Command implementation
// that handles removing a gossip encryption key from a keyring
type OperatorGossipKeyringRemoveCommand struct {
	Meta
}

func (c *OperatorGossipKeyringRemoveCommand) Help() string {
	helpText := `
Usage: nomad operator gossip keyring remove [options] <key>

  Remove the given key from the cluster. This operation may only be performed
  on keys which are not currently the primary key.

  This command can only be run against server nodes. It returns 0 if all nodes
  reply and there are no errors. If any node fails to reply or reports failure,
  the exit code will be 1.

  If ACLs are enabled, this command requires a token with the 'agent:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *OperatorGossipKeyringRemoveCommand) Synopsis() string {
	return "Remove a gossip encryption key"
}

func (c *OperatorGossipKeyringRemoveCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *OperatorGossipKeyringRemoveCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictAnything
}

func (c *OperatorGossipKeyringRemoveCommand) Name() string { return "operator gossip keyring remove" }

func (c *OperatorGossipKeyringRemoveCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("operator-gossip-keyring-remove", FlagSetClient)
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
	if len(args) != 1 {
		c.Ui.Error("This command requires one argument: <key>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	removeKey := args[0]

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating nomad cli client: %s", err))
		return 1
	}

	c.Ui.Output("Removing gossip encryption key...")
	_, err = client.Agent().RemoveKey(removeKey)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error: %s", err))
		return 1
	}
	return 0
}
