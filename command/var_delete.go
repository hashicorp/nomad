package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type VarDeleteCommand struct {
	Meta
}

func (c *VarDeleteCommand) Help() string {
	helpText := `
Usage: nomad var delete [options] <path>

  Delete is used to delete an existing secure variable.

  If ACLs are enabled, this command requires a token with the 'var:purge'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault)

	return strings.TrimSpace(helpText)
}

func (c *VarDeleteCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *VarDeleteCommand) AutocompleteArgs() complete.Predictor {
	return SecureVariablePathPredictor(c.Meta.Client)
}

func (c *VarDeleteCommand) Synopsis() string {
	return "Delete a secure variable"
}

func (c *VarDeleteCommand) Name() string { return "var delete" }

func (c *VarDeleteCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <path>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	path := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	_, err = client.SecureVariables().Delete(path, nil)
	// TODO: Manage Conflict result
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting secure variable: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully deleted secure variable %q!", path))
	return 0
}
