package command

import (
	"fmt"
	"strings"
)

type ContextUpdateCommand struct {
	Meta
}

func (c *ContextUpdateCommand) Help() string {
	helpText := `
Usage: nomad context update [options] [args]

  List is used to list the currently registered services.

  If ACLs are enabled, this command requires a token with the 'read-job'
  capabilities for the namespace of all services. Any namespaces that the token
  does not have access to will have its services filtered from the results.
`
	return strings.TrimSpace(helpText)
}

func (c *ContextUpdateCommand) Synopsis() string { return "Update an existing context configuration" }

func (c *ContextUpdateCommand) Name() string { return "context update" }

func (c *ContextUpdateCommand) Run(args []string) int {

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

	ctxName := args[0]

	ctxInfo, err := ctxStorage.Get(ctxName)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error detailing context: %v", err))
		return 1
	}

	//
	ctxInfo.Context.mergeMetaFlags(c.Meta)

	if err := ctxStorage.Set(ctxInfo.Context); err != nil {
		c.Ui.Error(fmt.Sprintf("Error updating context: %v", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Context %q successfully updated", args[0]))
	return 0
}
