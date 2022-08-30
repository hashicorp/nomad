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
Usage: nomad var purge [options] <path>

  Purge is used to permanently delete an existing variable.

  If ACLs are enabled, this command requires a token with the 'var:destroy'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Purge Options:

  -check-index
    If set, the variable is only purged if the server side version's modify
	index matches the provided value.
`

	return strings.TrimSpace(helpText)
}

func (c *VarPurgeCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *VarPurgeCommand) AutocompleteArgs() complete.Predictor {
	return VariablePathPredictor(c.Meta.Client)
}

func (c *VarPurgeCommand) Synopsis() string {
	return "Purge a variable"
}

func (c *VarPurgeCommand) Name() string { return "var purge" }

func (c *VarPurgeCommand) Run(args []string) int {
	var checkIndexStr string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&checkIndexStr, "check-index", "", "")

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

	// Parse the check-index
	checkIndex, enforce, err := parseCheckIndex(checkIndexStr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing check-index value %q: %v", checkIndexStr, err))
		return 1
	}

	path := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if enforce {
		_, err = client.Variables().CheckedDelete(path, checkIndex, nil)
		// TODO: Manage Conflict result; for now, will be caught in generic error handler later.
	} else {
		_, err = client.Variables().Delete(path, nil)
	}

	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error purging variable: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully purged variable %q!", path))
	return 0
}
