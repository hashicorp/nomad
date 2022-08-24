package command

import (
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

type TLSCACommand struct {
	Meta
}

func NewCA() *TLSCACommand {
	return &TLSCACommand{}
}

func (c *TLSCACommand) Help() string {
	helpText := `
Usage: nomad tls ca <subcommand> [options] filename-prefix

This command has subcommands for interacting with Certificate Authorities.

Here are some simple examples, and more detailed examples are available
in the subcommands or the documentation.

Create a CA

  $ nomad tls ca create
  ==> saved nomad-agent-ca.pem
  ==> saved nomad-agent-ca-key.pem

For more examples, ask for subcommand help or view the documentation.

`
	return strings.TrimSpace(helpText)
}

func (c *TLSCACommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *TLSCACommand) Synopsis() string {
	return "Helpers for managing CAs"
}

func (c *TLSCACommand) Name() string { return "tls ca" }

func (c *TLSCACommand) Run(args []string) int {
	return cli.RunResultHelp
}
