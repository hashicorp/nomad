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

// Ensure ACLBindingRuleCreateCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLBindingRuleCreateCommand{}

// ACLBindingRuleCreateCommand implements cli.Command.
type ACLBindingRuleCreateCommand struct {
	Meta

	description string
	authMethod  string
	selector    string
	bindType    string
	bindName    string
	json        bool
	tmpl        string
}

// Help satisfies the cli.Command Help function.
func (a *ACLBindingRuleCreateCommand) Help() string {
	helpText := `
Usage: nomad acl binding-rule create [options]

  Create is used to create new ACL binding rules. Use requires a management
  token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Create Options:

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
    "management", this should not be set.

  -json
    Output the ACL binding rule in a JSON format.

  -t
    Format and display the ACL binding rule using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (a *ACLBindingRuleCreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-description": complete.PredictAnything,
			"-auth-method": complete.PredictAnything,
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

func (a *ACLBindingRuleCreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLBindingRuleCreateCommand) Synopsis() string { return "Create a new ACL binding rule" }

// Name returns the name of this command.
func (a *ACLBindingRuleCreateCommand) Name() string { return "acl binding-rule create" }

// Run satisfies the cli.Command Run function.
func (a *ACLBindingRuleCreateCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.StringVar(&a.description, "description", "", "")
	flags.StringVar(&a.authMethod, "auth-method", "", "")
	flags.StringVar(&a.selector, "selector", "", "")
	flags.StringVar(&a.bindType, "bind-type", "", "")
	flags.StringVar(&a.bindName, "bind-name", "", "")
	flags.BoolVar(&a.json, "json", false, "")
	flags.StringVar(&a.tmpl, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments.
	if len(flags.Args()) != 0 {
		a.Ui.Error("This command takes no arguments")
		a.Ui.Error(commandErrorText(a))
		return 1
	}

	// Perform some basic validation on the submitted binding rule information
	// to avoid sending API and RPC requests which will fail basic validation.
	if a.authMethod == "" {
		a.Ui.Error("ACL binding rule auth method must be specified using the -auth-method flag")
		return 1
	}
	if a.bindType == "" {
		a.Ui.Error("ACL binding rule bind type must be specified using the -bind-type flag")
		return 1
	}

	// Set up the ACL binding rule with the passed parameters.
	aclBindingRule := api.ACLBindingRule{
		Description: a.description,
		AuthMethod:  a.authMethod,
		Selector:    a.selector,
		BindType:    a.bindType,
		BindName:    a.bindName,
	}

	// Get the HTTP client.
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Create the ACL binding rule via the API.
	rule, _, err := client.ACLBindingRules().Create(&aclBindingRule, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error creating ACL binding rule: %s", err))
		return 1
	}

	if a.json || len(a.tmpl) > 0 {
		out, err := Format(a.json, a.tmpl, rule)
		if err != nil {
			a.Ui.Error(err.Error())
			return 1
		}

		a.Ui.Output(out)
		return 0
	}

	a.Ui.Output(formatACLBindingRule(rule))
	return 0
}
