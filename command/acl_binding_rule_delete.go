// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLBindingRuleDeleteCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLBindingRuleDeleteCommand{}

// ACLBindingRuleDeleteCommand implements cli.Command.
type ACLBindingRuleDeleteCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (a *ACLBindingRuleDeleteCommand) Help() string {
	helpText := `
Usage: nomad acl binding-rule delete <acl_binding_rule_id>

  Delete is used to delete an existing ACL binding rule. Use requires a
  management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (a *ACLBindingRuleDeleteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (a *ACLBindingRuleDeleteCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLBindingRuleDeleteCommand) Synopsis() string { return "Delete an existing ACL binding rule" }

// Name returns the name of this command.
func (a *ACLBindingRuleDeleteCommand) Name() string { return "acl binding-rule delete" }

// Run satisfies the cli.Command Run function.
func (a *ACLBindingRuleDeleteCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that the last argument is the rule ID to delete.
	if len(flags.Args()) != 1 {
		a.Ui.Error("This command takes one argument: <acl_binding_rule_id>")
		a.Ui.Error(commandErrorText(a))
		return 1
	}

	aclBindingRuleID := flags.Args()[0]

	// Get the HTTP client.
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Delete the specified ACL binding rule.
	_, err = client.ACLBindingRules().Delete(aclBindingRuleID, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error deleting ACL binding rule: %s", err))
		return 1
	}

	// Give some feedback to indicate the deletion was successful.
	a.Ui.Output(fmt.Sprintf("ACL binding rule %s successfully deleted", aclBindingRuleID))
	return 0
}
