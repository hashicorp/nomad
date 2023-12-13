// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

// OperatorRootKeyringRemoveCommand is a Command
// implementation that handles removeing variables encryption
// keys from a keyring.
type OperatorRootKeyringRemoveCommand struct {
	Meta
}

func (c *OperatorRootKeyringRemoveCommand) Help() string {
	helpText := `
Usage: nomad operator root keyring remove [options] <key ID>

  Remove an encryption key from the cluster. This operation may only be
  performed on keys that are not the active key.

  If ACLs are enabled, this command requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *OperatorRootKeyringRemoveCommand) Synopsis() string {
	return "Removes a root encryption key"
}

func (c *OperatorRootKeyringRemoveCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *OperatorRootKeyringRemoveCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictAnything
}

func (c *OperatorRootKeyringRemoveCommand) Name() string {
	return "root keyring remove"
}

func (c *OperatorRootKeyringRemoveCommand) Run(args []string) int {
	var verbose bool

	flags := c.Meta.FlagSet("root keyring remove", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command requires one argument: <key ID>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	removeKey := args[0]

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating nomad cli client: %s", err))
		return 1
	}
	_, err = client.Keyring().Delete(&api.KeyringDeleteOptions{
		KeyID: removeKey,
	}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error: %s", err))
		return 1
	}
	c.Ui.Output(fmt.Sprintf("Removed encryption key %s", removeKey))
	return 0
}
