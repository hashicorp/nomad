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

// Ensure ACLRoleCreateCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLRoleCreateCommand{}

// ACLRoleCreateCommand implements cli.Command.
type ACLRoleCreateCommand struct {
	Meta

	name        string
	description string
	policyNames []string
	json        bool
	tmpl        string
}

// Help satisfies the cli.Command Help function.
func (a *ACLRoleCreateCommand) Help() string {
	helpText := `
Usage: nomad acl role create [options]

  Create is used to create new ACL roles. Use requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

ACL Create Options:

  -name
    Sets the human readable name for the ACL role. The name must be between
    1-128 characters and is a required parameter.

  -description
    A free form text description of the role that must not exceed 256
    characters.

  -policy
    Specifies a policy to associate with the role identified by their name. This
    flag can be specified multiple times and must be specified at least once.

  -json
    Output the ACL role in a JSON format.

  -t
    Format and display the ACL role using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (a *ACLRoleCreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-name":        complete.PredictAnything,
			"-description": complete.PredictAnything,
			"-policy":      complete.PredictAnything,
			"-json":        complete.PredictNothing,
			"-t":           complete.PredictAnything,
		})
}

func (a *ACLRoleCreateCommand) AutocompleteArgs() complete.Predictor { return complete.PredictNothing }

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLRoleCreateCommand) Synopsis() string { return "Create a new ACL role" }

// Name returns the name of this command.
func (a *ACLRoleCreateCommand) Name() string { return "acl role create" }

// Run satisfies the cli.Command Run function.
func (a *ACLRoleCreateCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.StringVar(&a.name, "name", "", "")
	flags.StringVar(&a.description, "description", "", "")
	flags.Var((funcVar)(func(s string) error {
		a.policyNames = append(a.policyNames, s)
		return nil
	}), "policy", "")
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

	// Perform some basic validation on the submitted role information to avoid
	// sending API and RPC requests which will fail basic validation.
	if a.name == "" {
		a.Ui.Error("ACL role name must be specified using the -name flag")
		return 1
	}
	if len(a.policyNames) < 1 {
		a.Ui.Error("At least one policy name must be specified using the -policy flag")
		return 1
	}

	// Set up the ACL with the passed parameters.
	aclRole := api.ACLRole{
		Name:        a.name,
		Description: a.description,
		Policies:    aclRolePolicyNamesToPolicyLinks(a.policyNames),
	}

	// Get the HTTP client.
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Create the ACL role via the API.
	role, _, err := client.ACLRoles().Create(&aclRole, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error creating ACL role: %s", err))
		return 1
	}

	if a.json || len(a.tmpl) > 0 {
		out, err := Format(a.json, a.tmpl, role)
		if err != nil {
			a.Ui.Error(err.Error())
			return 1
		}

		a.Ui.Output(out)
		return 0
	}

	a.Ui.Output(formatACLRole(role))
	return 0
}
