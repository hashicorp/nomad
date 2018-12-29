package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type ACLPolicyDeleteCommand struct {
	Meta
}

func (c *ACLPolicyDeleteCommand) Help() string {
	helpText := `
Usage: nomad acl policy delete <name>

  Delete is used to delete an existing ACL policy.

General Options:

  ` + generalOptionsUsage()

	return strings.TrimSpace(helpText)
}

func (c *ACLPolicyDeleteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *ACLPolicyDeleteCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLPolicyDeleteCommand) Synopsis() string {
	return "Delete an existing ACL policy"
}

func (c *ACLPolicyDeleteCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("acl policy delete", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the policy name
	policyName := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Delete the policy
	_, err = client.ACLPolicies().Delete(policyName, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting ACL policy: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully deleted %s policy!",
		policyName))
	return 0
}
