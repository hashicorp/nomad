// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// OperatorGossipKeyringUseCommand is a Command implementation that
// handles setting the gossip encryption key from a keyring
type OperatorGossipKeyringUseCommand struct {
	Meta
}

func (c *OperatorGossipKeyringUseCommand) Help() string {
	helpText := `
Usage: nomad operator gossip keyring use [options] <key>

  Change the encryption key used for gossip. The key must already be installed
  before this operator can succeed.

  This command can only be run against server nodes. It returns 0 if all nodes
  reply and there are no errors. If any node fails to reply or reports failure,
  the exit code will be 1.

  If ACLs are enabled, this command requires a token with the 'agent:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *OperatorGossipKeyringUseCommand) Synopsis() string {
	return "Change the gossip encryption key"
}

func (c *OperatorGossipKeyringUseCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *OperatorGossipKeyringUseCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictAnything
}

func (c *OperatorGossipKeyringUseCommand) Name() string { return "operator gossip keyring use" }

func (c *OperatorGossipKeyringUseCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("operator-gossip-keyring-use", FlagSetClient)
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
	useKey := args[0]

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating nomad cli client: %s", err))
		return 1
	}

	c.Ui.Output("Changing primary gossip encryption key...")
	_, err = client.Agent().UseKey(useKey)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error: %s", err))
		return 1
	}
	return 0
}
