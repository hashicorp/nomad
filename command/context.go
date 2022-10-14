package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type ContextCommand struct {
	Meta
}

func (c *ContextCommand) Help() string {
	helpText := `
Usage: nomad context <subcommand> [options] [args]

  This command groups subcommands for interacting with contexts. Nomad contexts
  are a CLI only feature to easily manage access to multiple environments.

  Create or overwrite a context:

      $ nomad context create -address="https://127.0.0.1:4646" <context-name>

  Delete a context:

      $ nomad context delete <context-name>

  List contexts:

      $ nomad context list

  Read a context:

      $ nomad context info <context-name>

  Update an existing context:

      $ nomad context update -address="https://127.0.0.1:4646" <context-name>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (c *ContextCommand) Synopsis() string { return "Interact and manage Nomad environment contexts" }

func (c *ContextCommand) Name() string { return "context" }

func (c *ContextCommand) Run(_ []string) int { return cli.RunResultHelp }

func metaFlagsToNomadContextConfig(m Meta) *ContextConfig {

	var cfg ContextConfig

	if m.flagAddress != "" {
		cfg.Address = m.flagAddress
	}
	if m.region != "" {
		cfg.Region = m.region
	}
	if m.namespace != "" {
		cfg.Namespace = m.namespace
	}
	if m.token != "" {
		cfg.Token = m.token
	}
	if m.caCert != "" {
		cfg.CACert = m.caCert
	}
	if m.caPath != "" {
		cfg.CAPath = m.caCert
	}
	if m.clientCert != "" {
		cfg.ClientCert = m.caCert
	}
	if m.clientKey != "" {
		cfg.ClientKey = m.caCert
	}
	if m.tlsServerName != "" {
		cfg.TLSServerName = m.caCert
	}
	cfg.Insecure = m.insecure

	return &cfg
}
