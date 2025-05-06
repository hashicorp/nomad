// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type ACLPolicySelfCommand struct {
	Meta

	json bool
	tmpl string
}

func (c *ACLPolicySelfCommand) Help() string {
	helpText := `
Usage: nomad acl policy self

  Self is used to fetch information about the policy assigned to the current workload identity.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

ACL List Options:

  -json
    Output the ACL policies in a JSON format.

  -t
    Format and display the ACL policies using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *ACLPolicySelfCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *ACLPolicySelfCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLPolicySelfCommand) Synopsis() string {
	return "Lookup self ACL policy assigned to the workload identity"
}

func (c *ACLPolicySelfCommand) Name() string { return "acl policy self" }

func (c *ACLPolicySelfCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&c.json, "json", false, "")
	flags.StringVar(&c.tmpl, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we have no arguments
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

	policies, _, err := client.ACLPolicies().Self(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error fetching WI policies: %s", err))
		return 1
	}

	if len(policies) == 0 {
		c.Ui.Output("No policies found for this identity.")
	} else {
		if c.json || len(c.tmpl) > 0 {
			out, err := Format(c.json, c.tmpl, policies)
			if err != nil {
				c.Ui.Error(err.Error())
				return 1
			}

			c.Ui.Output(out)
			return 0
		}

		output := make([]string, 0, len(policies)+1)
		output = append(output, "Name|Job ID|Group Name|Task Name")
		for _, p := range policies {
			var outputString string
			if p.JobACL == nil {
				outputString = fmt.Sprintf("%s|%s|%s|%s", p.Name, "<unavailable>", "<unavailable>", "<unavailable>")
			} else {
				outputString = fmt.Sprintf(
					"%s|%s|%s|%s",
					p.Name, formatJobACL(p.JobACL.JobID), formatJobACL(p.JobACL.Group), formatJobACL(p.JobACL.Task),
				)
			}
			output = append(output, outputString)
		}

		c.Ui.Output(formatList(output))
	}
	return 0
}

func formatJobACL(jobACL string) string {
	if jobACL == "" {
		return "<not specified>"
	}
	return jobACL
}
