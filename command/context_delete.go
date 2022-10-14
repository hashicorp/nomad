package command

import (
	"fmt"
	"strings"
)

type ContextDeleteCommand struct {
	Meta
}

func (c *ContextDeleteCommand) Help() string {
	helpText := `
Usage: nomad context delete [args]

  List is used to list the currently registered services.

  If ACLs are enabled, this command requires a token with the 'read-job'
  capabilities for the namespace of all services. Any namespaces that the token
  does not have access to will have its services filtered from the results.
`
	return strings.TrimSpace(helpText)
}

func (c *ContextDeleteCommand) Synopsis() string { return "Delete an existing context configuration" }

func (c *ContextDeleteCommand) Name() string { return "context delete" }

func (c *ContextDeleteCommand) Run(args []string) int {

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

	ctxName := args[0]

	ctxStorage, err := newMetaContextStorage()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error setting up context: %v", err))
		return 1
	}

	if err := ctxStorage.Delete(ctxName); err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting context: %v", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Context %q successfully deleted", ctxName))
	return 0
}
