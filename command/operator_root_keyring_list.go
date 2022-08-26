package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

// OperatorSecureVariablesKeyringListCommand is a Command
// implementation that lists the secure variables encryption keys.
type OperatorSecureVariablesKeyringListCommand struct {
	Meta
}

func (c *OperatorSecureVariablesKeyringListCommand) Help() string {
	helpText := `
Usage: nomad operator secure-variables keyring list [options]

  List the currently installed keys. This list returns key metadata and not
  sensitive key material.

  If ACLs are enabled, this command requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Keyring Options:

  -verbose
    Show full information.
`

	return strings.TrimSpace(helpText)
}

func (c *OperatorSecureVariablesKeyringListCommand) Synopsis() string {
	return "Lists the secure variables encryption keys"
}

func (c *OperatorSecureVariablesKeyringListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-verbose": complete.PredictNothing,
		})
}

func (c *OperatorSecureVariablesKeyringListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorSecureVariablesKeyringListCommand) Name() string {
	return "secure-variables keyring list"
}

func (c *OperatorSecureVariablesKeyringListCommand) Run(args []string) int {
	var verbose bool

	flags := c.Meta.FlagSet("secure-variables keyring list", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if len(args) != 0 {
		c.Ui.Error("This command requires no arguments.")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating nomad cli client: %s", err))
		return 1
	}

	resp, _, err := client.Keyring().List(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error: %s", err))
		return 1
	}
	c.Ui.Output(renderSecureVariablesKeysResponse(resp, verbose))
	return 0
}
