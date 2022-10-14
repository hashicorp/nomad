package command

import (
	"fmt"
	"strings"
)

type ContextDefaultUnsetCommand struct {
	Meta
}

func (c *ContextDefaultUnsetCommand) Help() string {
	helpText := `
Usage: nomad context default unset [options]

  List is used to list the currently registered services.

  If ACLs are enabled, this command requires a token with the 'read-job'
  capabilities for the namespace of all services. Any namespaces that the token
  does not have access to will have its services filtered from the results.
`
	return strings.TrimSpace(helpText)
}

func (c *ContextDefaultUnsetCommand) Synopsis() string { return "Remove any existing default context" }

func (c *ContextDefaultUnsetCommand) Name() string { return "context default unset" }

func (c *ContextDefaultUnsetCommand) Run(args []string) int {

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

	if err := ctxStorage.UnsetDefault(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error removing default context: %v", err))
		return 1
	}

	c.Ui.Output("Default context successfully removed")
	return 0
}
