// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLBindingRuleInfoCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLBindingRuleInfoCommand{}

// ACLBindingRuleInfoCommand implements cli.Command.
type ACLBindingRuleInfoCommand struct {
	Meta

	json bool
	tmpl string
}

// Help satisfies the cli.Command Help function.
func (a *ACLBindingRuleInfoCommand) Help() string {
	helpText := `
Usage: nomad acl binding-rule info [options] <acl_binding_rule_id>

  Info is used to fetch information on an existing ACL binding rule. Requires a
  management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Info Options:

  -json
    Output the ACL binding rule in a JSON format.

  -t
    Format and display the ACL binding rule using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (a *ACLBindingRuleInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (a *ACLBindingRuleInfoCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLBindingRuleInfoCommand) Synopsis() string {
	return "Fetch information on an existing ACL binding rule"
}

// Name returns the name of this command.
func (a *ACLBindingRuleInfoCommand) Name() string { return "acl binding-rule info" }

// Run satisfies the cli.Command Run function.
func (a *ACLBindingRuleInfoCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.BoolVar(&a.json, "json", false, "")
	flags.StringVar(&a.tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we have exactly one argument.
	if len(flags.Args()) != 1 {
		a.Ui.Error("This command takes one argument: <acl_binding_rule_id>")
		a.Ui.Error(commandErrorText(a))
		return 1
	}

	// Get the HTTP client.
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	aclBindingRule, _, err := client.ACLBindingRules().Get(flags.Args()[0], nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error reading ACL binding rule: %s", err))
		return 1
	}

	if a.json || len(a.tmpl) > 0 {
		out, err := Format(a.json, a.tmpl, aclBindingRule)
		if err != nil {
			a.Ui.Error(err.Error())
			return 1
		}

		a.Ui.Output(out)
		return 0
	}

	// Format the output.
	a.Ui.Output(formatACLBindingRule(aclBindingRule))
	return 0
}
