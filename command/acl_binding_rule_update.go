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

// Ensure ACLBindingRuleUpdateCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLBindingRuleUpdateCommand{}

// ACLBindingRuleUpdateCommand implements cli.Command.
type ACLBindingRuleUpdateCommand struct {
	Meta

	description string
	selector    string
	bindType    string
	bindName    string
	noMerge     bool
	json        bool
	tmpl        string
}

// Help satisfies the cli.Command Help function.
func (a *ACLBindingRuleUpdateCommand) Help() string {
	helpText := `
Usage: nomad acl binding-rule update [options] <acl_binding_rule_id>

  Update is used to update an existing ACL binding rule. Requires a management
  token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Update Options:

  -description
    A free form text description of the binding rule that must not exceed 256
    characters.

  -auth-method
    Specifies the name of the ACL authentication method that this binding rule
    is associated with.

  -selector
    Selector is an expression that matches against verified identity attributes
    returned from the auth method during login.

  -bind-type
    Specifies adjusts how this binding rule is applied at login time to internal
    Nomad objects. Valid options are "role", "policy", or "management".

  -bind-name
    Specifies is the target of the binding used on selector match. This can be
    lightly templated using HIL ${foo} syntax. If the bind type is set to
    management, this should not be set.

  -json
    Output the ACL binding rule in a JSON format.

  -t
    Format and display the ACL binding rule using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (a *ACLBindingRuleUpdateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-description": complete.PredictAnything,
			"-selector":    complete.PredictAnything,
			"-bind-type": complete.PredictSet(
				api.ACLBindingRuleBindTypeRole,
				api.ACLBindingRuleBindTypePolicy,
				api.ACLBindingRuleBindTypeManagement,
			),
			"-bind-name": complete.PredictAnything,
			"-json":      complete.PredictNothing,
			"-t":         complete.PredictAnything,
		})
}

func (a *ACLBindingRuleUpdateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLBindingRuleUpdateCommand) Synopsis() string { return "Update an existing ACL binding rule" }

// Name returns the name of this command.
func (*ACLBindingRuleUpdateCommand) Name() string { return "acl binding-rule update" }

// Run satisfies the cli.Command Run function.
func (a *ACLBindingRuleUpdateCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.StringVar(&a.description, "description", "", "")
	flags.StringVar(&a.selector, "selector", "", "")
	flags.StringVar(&a.bindType, "bind-type", "", "")
	flags.StringVar(&a.bindName, "bind-name", "", "")
	flags.BoolVar(&a.noMerge, "no-merge", false, "")
	flags.BoolVar(&a.json, "json", false, "")
	flags.StringVar(&a.tmpl, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one argument which is expected to be the ACL
	// binding rule ID.
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

	aclBindingRuleID := flags.Args()[0]

	// Read the current rule in both cases, so we can fail better if not found.
	currentACLBindingRule, _, err := client.ACLBindingRules().Get(aclBindingRuleID, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error when retrieving ACL binding rule: %v", err))
		return 1
	}

	var updatedRule api.ACLBindingRule

	// Depending on whether we are merging or not, we need to take a different
	// approach.
	switch a.noMerge {
	case true:

		if a.bindType == "" {
			a.Ui.Error("ACL binding rule bind type must be specified using the -bind-type flag")
			return 1
		}

		updatedRule = api.ACLBindingRule{
			ID:          currentACLBindingRule.ID,
			Description: a.description,
			AuthMethod:  currentACLBindingRule.AuthMethod,
			Selector:    a.selector,
			BindType:    a.bindType,
			BindName:    a.bindName,
		}
	default:
		// Check that the operator specified at least one flag to update the ACL
		// binding rule with.
		if a.description == "" && a.selector == "" && a.bindType == "" && a.bindName == "" {
			a.Ui.Error("Please provide at least one update for the ACL binding rule")
			a.Ui.Error(commandErrorText(a))
			return 1
		}

		updatedRule = *currentACLBindingRule

		// If the operator specified a name or description, overwrite the
		// existing value as these are simple strings.
		if a.description != "" {
			updatedRule.Description = a.description
		}
		if a.selector != "" {
			updatedRule.Selector = a.selector
		}
		if a.bindType != "" {
			updatedRule.BindType = a.bindType
		}
		if a.bindName != "" {
			updatedRule.BindName = a.bindName
		}
	}

	// Update the ACL binding rule with the new information via the API.
	updatedACLBindingRuleRead, _, err := client.ACLBindingRules().Update(&updatedRule, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error updating ACL binding rule: %s", err))
		return 1
	}

	if a.json || len(a.tmpl) > 0 {
		out, err := Format(a.json, a.tmpl, updatedACLBindingRuleRead)
		if err != nil {
			a.Ui.Error(err.Error())
			return 1
		}

		a.Ui.Output(out)
		return 0
	}

	// Format the output
	a.Ui.Output(formatACLBindingRule(updatedACLBindingRuleRead))
	return 0
}
