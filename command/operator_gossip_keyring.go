// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// OperatorGossipKeyringCommand is a Command implementation that
// handles querying, installing, and removing gossip encryption keys
// from a keyring.
type OperatorGossipKeyringCommand struct {
	Meta
}

func (c *OperatorGossipKeyringCommand) Help() string {
	helpText := `
Usage: nomad operator gossip keyring [options]

  Manages encryption keys used for gossip messages between Nomad servers. Gossip
  encryption is optional. When enabled, this command may be used to examine
  active encryption keys in the cluster, add new keys, and remove old ones. When
  combined, this functionality provides the ability to perform key rotation
  cluster-wide, without disrupting the cluster.

  Generate an encryption key:

      $ nomad operator gossip keyring generate

  List all gossip encryption keys:

      $ nomad operator gossip keyring list

  Remove an encryption key from the keyring:

      $ nomad operator gossip keyring remove <key>

  Install an encryption key from backup:

      $ nomad operator gossip keyring install <key>

  Use an already-installed encryption key:

      $ nomad operator gossip keyring use <key>

  Please see individual subcommand help for detailed usage information.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *OperatorGossipKeyringCommand) Synopsis() string {
	return "Manages gossip layer encryption keys"
}

func (c *OperatorGossipKeyringCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *OperatorGossipKeyringCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorGossipKeyringCommand) Name() string { return "operator gossip keyring" }

func (c *OperatorGossipKeyringCommand) Run(args []string) int {
	return cli.RunResultHelp
}
