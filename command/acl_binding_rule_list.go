// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLBindingRuleListCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLBindingRuleListCommand{}

// ACLBindingRuleListCommand implements cli.Command.
type ACLBindingRuleListCommand struct {
	Meta

	json bool
	tmpl string
}

// Help satisfies the cli.Command Help function.
func (a *ACLBindingRuleListCommand) Help() string {
	helpText := `
Usage: nomad acl binding-rule list [options]

  List is used to list existing ACL binding rules. Requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

List Options:

  -json
    Output the ACL binding rules in a JSON format.

  -t
    Format and display the ACL binding rules using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (a *ACLBindingRuleListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (a *ACLBindingRuleListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLBindingRuleListCommand) Synopsis() string { return "List ACL binding rules" }

// Name returns the name of this command.
func (a *ACLBindingRuleListCommand) Name() string { return "acl binding-rule list" }

// Run satisfies the cli.Command Run function.
func (a *ACLBindingRuleListCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.BoolVar(&a.json, "json", false, "")
	flags.StringVar(&a.tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	if len(flags.Args()) != 0 {
		a.Ui.Error("This command takes no arguments")
		a.Ui.Error(commandErrorText(a))
		return 1
	}

	// Get the HTTP client
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Fetch info on the policy
	aclBindingRules, _, err := client.ACLBindingRules().List(nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error listing ACL binding rules: %s", err))
		return 1
	}

	if a.json || len(a.tmpl) > 0 {
		out, err := Format(a.json, a.tmpl, aclBindingRules)
		if err != nil {
			a.Ui.Error(err.Error())
			return 1
		}

		a.Ui.Output(out)
		return 0
	}

	a.Ui.Output(formatACLBindingRules(aclBindingRules))
	return 0
}

func formatACLBindingRules(rules []*api.ACLBindingRuleListStub) string {
	if len(rules) == 0 {
		return "No ACL binding rules found"
	}

	output := make([]string, 0, len(rules)+1)
	output = append(output, "ID|Description|Auth Method")
	for _, rule := range rules {
		output = append(output, fmt.Sprintf("%s|%s|%s", rule.ID, rule.Description, rule.AuthMethod))
	}

	return formatList(output)
}
