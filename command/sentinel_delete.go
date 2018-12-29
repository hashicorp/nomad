package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type SentinelDeleteCommand struct {
	Meta
}

func (c *SentinelDeleteCommand) Help() string {
	helpText := `
Usage: nomad sentinel delete [options] <name>

  Delete is used to delete an existing Sentinel policy.

General Options:

  ` + generalOptionsUsage() + `

`
	return strings.TrimSpace(helpText)
}

func (c *SentinelDeleteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *SentinelDeleteCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *SentinelDeleteCommand) Synopsis() string {
	return "Delete an existing Sentinel policies"
}

func (c *SentinelDeleteCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("sentinel delete", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one arguments
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the name and file
	policyName := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Get the list of policies
	_, err = client.SentinelPolicies().Delete(policyName, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting Sentinel policy: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully deleted %q Sentinel policy!",
		policyName))
	return 0
}
