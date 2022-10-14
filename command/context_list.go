package command

import (
	"fmt"
	"strings"
)

type ContextListCommand struct {
	Meta
}

func (c *ContextListCommand) Help() string {
	helpText := `
Usage: nomad context list

  List is used to list the currently registered services.

  If ACLs are enabled, this command requires a token with the 'read-job'
  capabilities for the namespace of all services. Any namespaces that the token
  does not have access to will have its services filtered from the results.
`
	return strings.TrimSpace(helpText)
}

func (c *ContextListCommand) Synopsis() string { return "List all contexts available to the CLI" }

func (c *ContextListCommand) Name() string { return "context list" }

func (c *ContextListCommand) Run(args []string) int {

	flagSet := c.Meta.FlagSet(c.Name(), FlagSetNone)
	flagSet.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flagSet.Parse(args); err != nil {
		return 1
	}

	args = flagSet.Args()
	if len(args) > 0 {
		c.Ui.Error("This command takes no arguments")
		return 1
	}

	ctxStorage, err := newMetaContextStorage()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error setting up context: %v", err))
		return 1
	}

	ctxList, err := ctxStorage.List()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error listing available contexts: %v", err))
		return 1
	}

	c.Ui.Output(formatContexts(ctxList))
	return 0
}

func formatContexts(cfgs []*MetaContextConfig) string {
	if len(cfgs) == 0 {
		return "No contexts found"
	}

	rows := make([]string, len(cfgs)+1)
	rows[0] = "Name|Address|Region"
	for i, cfg := range cfgs {
		rows[i+1] = fmt.Sprintf("%s|%s|%s",
			cfg.Context.Name, cfg.Context.Address, cfg.Context.Region)
	}
	return formatList(rows)
}
