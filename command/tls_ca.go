// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

type TLSCACommand struct {
	Meta
}

func (c *TLSCACommand) Help() string {
	helpText := `
Usage: nomad tls ca <subcommand> [options]

  This command groups subcommands for interacting with certificate authorities.
  For examples, see the documentation.

  Create a certificate authority.

      $ nomad tls ca create

  Show information about a certificate authority.

      $ nomad tls ca info
`
	return strings.TrimSpace(helpText)
}

func (c *TLSCACommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *TLSCACommand) Synopsis() string {
	return "Helpers for managing certificate authorities"
}

func (c *TLSCACommand) Name() string { return "tls ca" }

func (c *TLSCACommand) Run(_ []string) int {
	return cli.RunResultHelp
}
