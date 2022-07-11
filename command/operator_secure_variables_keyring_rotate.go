package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

// OperatorSecureVariablesKeyringRotateCommand is a Command
// implementation that rotates the secure variables encryption key.
type OperatorSecureVariablesKeyringRotateCommand struct {
	Meta
}

func (c *OperatorSecureVariablesKeyringRotateCommand) Help() string {
	helpText := `
Usage: nomad operator secure-variables keyring rotate [options]

  Generate a new encryption key for all future variables.

  If ACLs are enabled, this command requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Keyring Options:

  -full
    Decrypt all existing variables and re-encrypt with the new key. This command
    will immediately return and the re-encryption process will run
    asynchronously on the leader.

  -verbose
    Show full information.
`

	return strings.TrimSpace(helpText)
}

func (c *OperatorSecureVariablesKeyringRotateCommand) Synopsis() string {
	return "Rotates the secure variables encryption key"
}

func (c *OperatorSecureVariablesKeyringRotateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-full":    complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *OperatorSecureVariablesKeyringRotateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorSecureVariablesKeyringRotateCommand) Name() string {
	return "secure-variables keyring rotate"
}

func (c *OperatorSecureVariablesKeyringRotateCommand) Run(args []string) int {
	var rotateFull, verbose bool

	flags := c.Meta.FlagSet("secure-variables keyring rotate", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&rotateFull, "full", false, "full key rotation")
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

	resp, _, err := client.Keyring().Rotate(
		&api.KeyringRotateOptions{Full: rotateFull}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error: %s", err))
		return 1
	}
	c.Ui.Output(renderSecureVariablesKeysResponse([]*api.RootKeyMeta{resp}, verbose))
	return 0
}
