package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type ACLPolicyInfoCommand struct {
	Meta
}

func (c *ACLPolicyInfoCommand) Help() string {
	helpText := `
Usage: nomad acl policy info <name>

  Info is used to fetch information on an existing ACL policy.

General Options:

  ` + generalOptionsUsage()

	return strings.TrimSpace(helpText)
}

func (c *ACLPolicyInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *ACLPolicyInfoCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLPolicyInfoCommand) Synopsis() string {
	return "Fetch info on an existing ACL policy"
}

func (c *ACLPolicyInfoCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("acl policy info", FlagSetClient)
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

	// Fetch info on the policy
	policy, _, err := client.ACLPolicies().Info(policyName, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error fetching info on ACL policy: %s", err))
		return 1
	}

	c.Ui.Output(formatKVPolicy(policy))
	return 0
}
