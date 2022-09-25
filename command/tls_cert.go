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
  ==> WARNING: Server Certificates grants authority to become a
    server and access all state in the cluster including root keys
    and all ACL tokens. Do not distribute them to production hosts
    that are not server nodes. Store them as securely as CA keys.
  ==> Using CA file nomad-agent-ca.pem and CA key nomad-agent-ca-key.pem
  ==> Server Certificate saved to global-server-nomad.pem
  ==> Server Certificate key saved to global-server-nomad-key.pem

Create a certificate with your own CA:

  $ nomad tls cert create -server -ca my-ca.pem -key my-ca-key.pem
  ==> WARNING: Server Certificates grants authority to become a
    server and access all state in the cluster including root keys
    and all ACL tokens. Do not distribute them to production hosts
    that are not server nodes. Store them as securely as CA keys.
  ==> Using CA file my-ca.pem and CA key my-ca-key.pem
  ==> Server Certificate saved to global-server-nomad-1.pem
  ==> Server Certificate key saved to global-server-nomad-1-key.pem

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
