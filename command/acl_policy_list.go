// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

  This command requires a management ACL token to view all policies. A
  non-management token can query its own policies.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

List Options:

  -json
    Output the ACL policies in a JSON format.

  -t
    Format and display the ACL policies using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (c *ACLPolicyListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *ACLPolicyListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLPolicyListCommand) Synopsis() string {
	return "List ACL policies"
}

func (c *ACLPolicyListCommand) Name() string { return "acl policy list" }

func (c *ACLPolicyListCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
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

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, policies)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatPolicies(policies))
	return 0
}

func formatPolicies(policies []*api.ACLPolicyListStub) string {
	if len(policies) == 0 {
		return "No policies found"
	}

	output := make([]string, 0, len(policies)+1)
	output = append(output, "Name|Description")
	for _, p := range policies {
		output = append(output, fmt.Sprintf("%s|%s", p.Name, p.Description))
	}

	return formatList(output)
}
