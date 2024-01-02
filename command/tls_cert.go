// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

type TLSCertCommand struct {
	Meta
}

func (c *TLSCertCommand) Help() string {
	helpText := `
Usage: nomad tls cert <subcommand> [options]

  This command groups subcommands for interacting with certificates.
  For examples, see the documentation.

  Create a TLS certificate.

      $ nomad tls cert create

  Show information about a TLS certificate.

      $ nomad tls cert info
`
	return strings.TrimSpace(helpText)
}

func (c *TLSCertCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *TLSCertCommand) Synopsis() string {
	return "Helpers for managing certificates"
}

func (c *TLSCertCommand) Name() string { return "tls cert" }

func (c *TLSCertCommand) Run(_ []string) int {
	return cli.RunResultHelp
}
