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
Usage: nomad tls cert <subcommand> [options] [filename-prefix]

This command has subcommands for interacting with certificates.

Here are some simple examples, and more detailed examples are available
in the subcommands or the documentation.

Create a certificate

  $ nomad tls cert create -server
  ==> Server Certificate saved to: dc1-server-nomad.pem
  ==> Server Certificate key saved to: dc1-server-nomad-key.pem

Create a certificate with your own CA:

  $ nomad tls cert create -server -ca my-ca.pem -key my-ca-key.pem
  ==> Server Certificate saved to: dc1-server-nomad.pem
  ==> Server Certificate key saved to: dc1-server-nomad-key.pem

For more examples, ask for subcommand help or view the documentation.

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
