package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type VarPurgeCommand struct {
	Meta
}

func (c *VarPurgeCommand) Help() string {
	helpText := `
Usage: nomad var delete [options] <path>

  Delete is used to delete an existing variable.

  If ACLs are enabled, this command requires a token with the 'var:destroy'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault)

	return strings.TrimSpace(helpText)
}

func (c *VarPurgeCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *VarPurgeCommand) AutocompleteArgs() complete.Predictor {
	return VariablePathPredictor(c.Meta.Client)
}

func (c *VarPurgeCommand) Synopsis() string {
	return "Delete a variable"
}

func (c *VarPurgeCommand) Name() string { return "var delete" }

func (c *VarPurgeCommand) Run(args []string) int {
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

	_, err = client.Variables().Delete(path, nil)
	// TODO: Manage Conflict result
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting variable: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully deleted variable %q!", path))
	return 0
}
