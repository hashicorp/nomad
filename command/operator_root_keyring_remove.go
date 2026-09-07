// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
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

  If ACLs are enabled, this command requires a token with the operator:write
  policy or the operator:keyring-delete capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Remove Options:

  -force
    Remove the key even if it was used to sign an existing variable
    or workload identity.
`

	return strings.TrimSpace(helpText)
}

func (c *OperatorRootKeyringRemoveCommand) Synopsis() string {
	return "Removes a root encryption key"
}

func (c *OperatorRootKeyringRemoveCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-force": complete.PredictNothing,
		})
}

func (c *OperatorRootKeyringRemoveCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictAnything
}

func (c *OperatorRootKeyringRemoveCommand) Name() string {
	return "root keyring remove"
}

func (c *OperatorRootKeyringRemoveCommand) Run(args []string) int {
	var force bool
	flags := c.Meta.FlagSet("root keyring remove", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&force, "force", false, "Forces deletion of the root keyring even if it's in use.")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if len(args) != 1 || args[0] == "" {
		c.Ui.Error("This command requires one argument: <key ID>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	removeKey := strings.ToLower(args[0])

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating nomad cli client: %s", err))
		return 1
	}

	// Resolve an abbreviated key ID to the full ID
	if !helper.IsUUID(removeKey) {
		keys, _, err := client.Keyring().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}

		var matches []*api.RootKeyMeta
		for _, k := range keys {
			if strings.HasPrefix(k.KeyID, removeKey) {
				matches = append(matches, k)
			}
		}

		if len(matches) == 0 {
			c.Ui.Error(fmt.Sprintf("No encryption key(s) with prefix or ID %q found", removeKey))
			return 1
		}

		if len(matches) > 1 {
			c.Ui.Error(fmt.Sprintf("Prefix matched multiple keys\n\n%s",
				renderVariablesKeysResponse(matches, true)))
			return 1
		}

		removeKey = matches[0].KeyID
	}

	_, err = client.Keyring().Delete(&api.KeyringDeleteOptions{
		KeyID: removeKey,
		Force: force,
	}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error: %s", err))
		return 1
	}
	c.Ui.Output(fmt.Sprintf("Removed encryption key %s", removeKey))
	return 0
}
