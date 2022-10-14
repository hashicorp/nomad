package command

import (
	"fmt"
	"strings"
)

type ContextInfoCommand struct {
	Meta
}

func (c *ContextInfoCommand) Help() string {
	helpText := `
Usage: nomad context info [args]

  List is used to list the currently registered services.

  If ACLs are enabled, this command requires a token with the 'read-job'
  capabilities for the namespace of all services. Any namespaces that the token
  does not have access to will have its services filtered from the results.
`
	return strings.TrimSpace(helpText)
}

func (c *ContextInfoCommand) Synopsis() string { return "Read a context configuration" }

func (c *ContextInfoCommand) Name() string { return "context info" }

func (c *ContextInfoCommand) Run(args []string) int {

	flagSet := c.Meta.FlagSet(c.Name(), FlagSetNone)
	flagSet.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flagSet.Parse(args); err != nil {
		return 1
	}

	args = flagSet.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <context-name>")
		return 1
	}

	ctxStorage, err := newMetaContextStorage()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error setting up context: %v", err))
		return 1
	}

	ctxInfo, err := ctxStorage.Get(args[0])
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error reading context: %v", err))
		return 1
	}

	c.Ui.Output(formatContext(ctxInfo))
	return 0
}

func formatContext(cfg *MetaContextConfig) string {
	return formatKV([]string{
		fmt.Sprintf("Name|%s", cfg.Context.Name),
		fmt.Sprintf("Address|%s", cfg.Context.Address),
		fmt.Sprintf("Region|%s", cfg.Context.Region),
		fmt.Sprintf("Namespace|%s", cfg.Context.Namespace),
		fmt.Sprintf("Token|%s", cfg.Context.Token),
		fmt.Sprintf("CA Cert|%s", cfg.Context.CACert),
		fmt.Sprintf("CA Path|%s", cfg.Context.CAPath),
		fmt.Sprintf("Client Cert|%s", cfg.Context.ClientCert),
		fmt.Sprintf("Client Key|%s", cfg.Context.ClientKey),
		fmt.Sprintf("TLS Server Name|%s", cfg.Context.TLSServerName),
		fmt.Sprintf("Insecure|%v", cfg.Context.Insecure),
	})
}
