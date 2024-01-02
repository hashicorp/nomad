// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// OperatorGossipKeyringInstallCommand is a Command implementation
// that handles installing a gossip encryption key from a keyring
type OperatorGossipKeyringInstallCommand struct {
	Meta
}

func (c *OperatorGossipKeyringInstallCommand) Help() string {
	helpText := `
Usage: nomad operator gossip keyring install [options] <key>

  Install a new encryption key used for gossip. This will broadcast the new key
  to all members in the cluster.

  This command can only be run against server nodes. It returns 0 if all nodes
  reply and there are no errors. If any node fails to reply or reports failure,
  the exit code will be 1.

  If ACLs are enabled, this command requires a token with the 'agent:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *OperatorGossipKeyringInstallCommand) Synopsis() string {
	return "Install a gossip encryption key"
}

func (c *OperatorGossipKeyringInstallCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *OperatorGossipKeyringInstallCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictAnything
}

func (c *OperatorGossipKeyringInstallCommand) Name() string { return "operator gossip keyring install" }

func (c *OperatorGossipKeyringInstallCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("operator-gossip-keyring-install", FlagSetClient)
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
	installKey := args[0]

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating nomad cli client: %s", err))
		return 1
	}

	c.Ui.Output("Installing new gossip encryption key...")
	_, err = client.Agent().InstallKey(installKey)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error: %s", err))
		return 1
	}
	return 0
}
