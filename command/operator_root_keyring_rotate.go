// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

// OperatorRootKeyringRotateCommand is a Command
// implementation that rotates the variables encryption key.
type OperatorRootKeyringRotateCommand struct {
	Meta
}

func (c *OperatorRootKeyringRotateCommand) Help() string {
	helpText := `
Usage: nomad operator root keyring rotate [options]

  Generate a new encryption key for all future variables.

  If ACLs are enabled, this command requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Keyring Options:

  -full
    Decrypt all existing variables and re-encrypt with the new key. This command
    will immediately return and the re-encryption process will run
    asynchronously on the leader.

  -now
    Publish the new key immediately without prepublishing. One of -now or
    -prepublish must be set.

  -prepublish
    Set a duration for which to prepublish the new key (ex. "1h"). The currently
    active key will be unchanged but the new public key will be available in the
    JWKS endpoint. Multiple keys can be prepublished and they will be promoted to
    active in order of publish time, at most once every root_key_gc_interval. One
    of -now or -prepublish must be set.

  -verbose
    Show full information.
`

	return strings.TrimSpace(helpText)
}

func (c *OperatorRootKeyringRotateCommand) Synopsis() string {
	return "Rotates the root encryption key"
}

func (c *OperatorRootKeyringRotateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-full":       complete.PredictNothing,
			"-now":        complete.PredictNothing,
			"-prepublish": complete.PredictNothing,
			"-verbose":    complete.PredictNothing,
		})
}

func (c *OperatorRootKeyringRotateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorRootKeyringRotateCommand) Name() string {
	return "root keyring rotate"
}

func (c *OperatorRootKeyringRotateCommand) Run(args []string) int {
	var rotateFull, rotateNow, verbose bool
	var prepublishDuration time.Duration

	flags := c.Meta.FlagSet("root keyring rotate", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&rotateFull, "full", false, "full key rotation")
	flags.BoolVar(&rotateNow, "now", false, "immediately rotate without prepublish")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.DurationVar(&prepublishDuration, "prepublish", 0, "prepublish key")

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

	if !rotateNow && prepublishDuration == 0 || rotateNow && prepublishDuration != 0 {
		c.Ui.Error(`
One of "-now" or "-prepublish" must be used.

If a key has been leaked use "-now" to force immediate rotation.

Otherwise please use "-prepublish <duration>" to ensure the new key is not used
to sign workload identities before JWKS endpoints are updated.
`)
		return 1
	}

	publishTime := int64(0)
	if prepublishDuration > 0 {
		publishTime = time.Now().UnixNano() + prepublishDuration.Nanoseconds()
	}

	resp, _, err := client.Keyring().Rotate(
		&api.KeyringRotateOptions{
			Full:        rotateFull,
			PublishTime: publishTime,
		}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error: %s", err))
		return 1
	}
	c.Ui.Output(renderVariablesKeysResponse([]*api.RootKeyMeta{resp}, verbose))
	return 0
}
