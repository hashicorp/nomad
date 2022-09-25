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

This command has subcommands for interacting with Certificate Authorities.

Here is a simple example, and more detailed examples are available
in the subcommands or the documentation.

Create a CA

  $ nomad tls ca create
  ==> CA Certificate saved to: nomad-agent-ca.pem
  ==> CA Certificate key saved to: nomad-agent-ca-key.pem
  $ nomad tls ca info nomad-agent-ca.pem
  nomad-agent-ca.pem
  Issuer CN              Nomad Agent CA 58896012363767591697986789371079092261
  Common Name            CN=Nomad Agent CA 58896012363767591697986789371079092261,O=HashiCorp Inc.,...
  Expiry Date            2027-09-24 22:24:08 +0000 UTC
  Permitted DNS Domains  []

For more examples, ask for subcommand help or view the documentation.

`
	return strings.TrimSpace(helpText)
}

func (c *TLSCACommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *TLSCACommand) Synopsis() string {
	return "Helpers for creating CAs"
}

func (c *TLSCACommand) Name() string { return "tls ca" }

func (c *TLSCACommand) Run(_ []string) int {
	return cli.RunResultHelp
}
