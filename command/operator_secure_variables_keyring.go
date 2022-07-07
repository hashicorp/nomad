package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"

	"github.com/hashicorp/nomad/api"
)

// OperatorSecureVariablesKeyringCommand is a Command implementation
// that handles querying, installing, and removing secure variables
// encryption keys from a keyring.
type OperatorSecureVariablesKeyringCommand struct {
	Meta
}

func (c *OperatorSecureVariablesKeyringCommand) Help() string {
	helpText := `
Usage: nomad operator secure-variables keyring [options]

  Manages encryption keys used for storing secure variables. This command may be
  used to examine active encryption keys in the cluster, rotate keys, add new
  keys from backups, or remove unused keys.

  If ACLs are enabled, all subcommands requires a management token.

  Rotate the encryption key:

      $ nomad operator secure-variables keyring rotate

  List all encryption key metadata:

      $ nomad operator secure-variables keyring list

  Remove an encryption key from the keyring:

      $ nomad operator secure-variables keyring remove <key ID>

  Install an encryption key from backup:

      $ nomad operator secure-variables keyring install <path to .json file>

  Please see individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorSecureVariablesKeyringCommand) Synopsis() string {
	return "Manages secure variables encryption keys"
}

func (c *OperatorSecureVariablesKeyringCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *OperatorSecureVariablesKeyringCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorSecureVariablesKeyringCommand) Name() string {
	return "secure-variables keyring"
}

func (c *OperatorSecureVariablesKeyringCommand) Run(args []string) int {
	return cli.RunResultHelp
}

// renderSecureVariablesKeysResponse is a helper for formatting the
// keyring API responses
func renderSecureVariablesKeysResponse(keys []*api.RootKeyMeta, verbose bool) string {
	length := fullId
	if !verbose {
		length = 8
	}
	out := make([]string, len(keys)+1)
	out[0] = "Key|State|Create Time"
	i := 1
	for _, k := range keys {
		out[i] = fmt.Sprintf("%s|%v|%s",
			k.KeyID[:length], k.State, formatTime(k.CreateTime))
		i = i + 1
	}
	return formatList(out)
}
