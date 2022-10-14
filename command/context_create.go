package command

import (
	"fmt"
	"strings"
)

type ContextCreateCommand struct {
	Meta
}

func (c *ContextCreateCommand) Help() string {
	helpText := `
Usage: nomad context list [options]

  List is used to list the currently registered services.

  If ACLs are enabled, this command requires a token with the 'read-job'
  capabilities for the namespace of all services. Any namespaces that the token
  does not have access to will have its services filtered from the results.
`
	return strings.TrimSpace(helpText)
}

func (c *ContextCreateCommand) Synopsis() string { return "Create or overwrite a context" }

func (c *ContextCreateCommand) Name() string { return "context create" }

func (c *ContextCreateCommand) Run(args []string) int {

	flagSet := c.Meta.FlagSet(c.Name(), FlagSetClient)
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

	ctxConfig := metaFlagsToNomadContextConfig(c.Meta)
	ctxConfig.Name = args[0]

	if err := ctxStorage.Set(ctxConfig); err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating context: %v", err))
	}

	c.Ui.Output(fmt.Sprintf("Context %q successfully created", ctxConfig.Name))
	return 0
}
