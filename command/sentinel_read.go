package command

import (
	"fmt"
	"strings"
)

type SentinelReadCommand struct {
	Meta
}

func (c *SentinelReadCommand) Help() string {
	helpText := `
Usage: nomad sentinel read [options] <name>

Read is used to inspect a Sentinel policy.

General Options:

  ` + generalOptionsUsage() + `

Read Options:

  -raw
    Prints only the raw policy

`
	return strings.TrimSpace(helpText)
}

func (c *SentinelReadCommand) Synopsis() string {
	return "Inspects an existing Sentinel policies"
}

func (c *SentinelReadCommand) Run(args []string) int {
	var raw bool
	flags := c.Meta.FlagSet("sentinel read", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&raw, "raw", false, "")
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

	// Query the policy
	policy, _, err := client.SentinelPolicies().Info(policyName, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying Sentinel policy: %s", err))
		return 1
	}

	// Check for only the raw policy
	if raw {
		c.Ui.Output(policy.Policy)
		return 0
	}

	// Output the base information
	info := []string{
		fmt.Sprintf("Name|%s", policy.Name),
		fmt.Sprintf("Scope|%s", policy.Scope),
		fmt.Sprintf("Enforcement Level|%s", policy.EnforcementLevel),
		fmt.Sprintf("Description|%s", policy.Description),
	}
	c.Ui.Output(formatKV(info))
	c.Ui.Output("Policy:")
	c.Ui.Output(policy.Policy)
	return 0
}
