// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"

	"github.com/hashicorp/nomad/api"
)

// OperatorRootKeyringCommand is a Command implementation
// that handles querying, rotating, and removing root
// encryption keys from a keyring.
type OperatorRootKeyringCommand struct {
	Meta
}

func (c *OperatorRootKeyringCommand) Help() string {
	helpText := `
Usage: nomad operator root keyring [options]

  Manages encryption keys used for storing variables and signing workload
  identities. This command may be used to examine active encryption keys
  in the cluster, rotate keys, add new keys from backups, or remove unused keys.

  If ACLs are enabled, all subcommands requires a management token.

  Rotate the encryption key:

      $ nomad operator root keyring rotate

  List all encryption key metadata:

      $ nomad operator root keyring list

  Remove an encryption key from the keyring:

      $ nomad operator root keyring remove <key ID>

  Please see individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRootKeyringCommand) Synopsis() string {
	return "Manages root encryption keys"
}

func (c *OperatorRootKeyringCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *OperatorRootKeyringCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorRootKeyringCommand) Name() string {
	return "root keyring"
}

func (c *OperatorRootKeyringCommand) Run(args []string) int {
	return cli.RunResultHelp
}

// renderVariablesKeysResponse is a helper for formatting the
// keyring API responses
func renderVariablesKeysResponse(keys []*api.RootKeyMeta, verbose bool) string {
	length := fullId
	if !verbose {
		length = 8
	}
	out := make([]string, len(keys)+1)
	out[0] = "Key|State|Create Time"
	i := 1
	for _, k := range keys {
		out[i] = fmt.Sprintf("%s|%v|%s",
			k.KeyID[:length], k.State, formatUnixNanoTime(k.CreateTime))
		i = i + 1
	}
	return formatList(out)
}
