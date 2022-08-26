package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

// OperatorSecureVariablesKeyringRemoveCommand is a Command
// implementation that handles removeing secure variables encryption
// keys from a keyring.
type OperatorSecureVariablesKeyringRemoveCommand struct {
	Meta
}

func (c *OperatorSecureVariablesKeyringRemoveCommand) Help() string {
	helpText := `
Usage: nomad operator secure-variables keyring remove [options] <key ID>

  Remove an encryption key from the cluster. This operation may only be
  performed on keys that are not the active key.

  If ACLs are enabled, this command requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *OperatorSecureVariablesKeyringRemoveCommand) Synopsis() string {
	return "Removes a secure variables encryption key"
}

func (c *OperatorSecureVariablesKeyringRemoveCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *OperatorSecureVariablesKeyringRemoveCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictAnything
}

func (c *OperatorSecureVariablesKeyringRemoveCommand) Name() string {
	return "secure-variables keyring remove"
}

func (c *OperatorSecureVariablesKeyringRemoveCommand) Run(args []string) int {
	var verbose bool

	flags := c.Meta.FlagSet("secure-variables keyring remove", FlagSetClient)
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
