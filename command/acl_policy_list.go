package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type ACLPolicyListCommand struct {
	Meta
}

func (c *ACLPolicyListCommand) Help() string {
	helpText := `
Usage: nomad acl policy list

List is used to list available ACL policies.

General Options:

  ` + generalOptionsUsage()

	return strings.TrimSpace(helpText)
}

func (c *ACLPolicyListCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *ACLPolicyListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLPolicyListCommand) Synopsis() string {
	return "List ACL policies"
}

func (c *ACLPolicyListCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("acl policy list", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Fetch info on the policy
	policies, _, err := client.ACLPolicies().List(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error listing ACL policies: %s", err))
		return 1
	}

	c.Ui.Output(formatPolicies(policies))
	return 0
}

func formatPolicies(policies []*api.ACLPolicyListStub) string {
	if len(policies) == 0 {
		return "No policies found"
	}

	output := make([]string, 0, len(policies)+1)
	output = append(output, fmt.Sprintf("Name|Description"))
	for _, p := range policies {
		output = append(output, fmt.Sprintf("%s|%s", p.Name, p.Description))
	}

	return formatList(output)
}
