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

// Ensure ACLRoleListCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLRoleListCommand{}

// ACLRoleListCommand implements cli.Command.
type ACLRoleListCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (a *ACLRoleListCommand) Help() string {
	helpText := `
Usage: nomad acl role list [options]

  List is used to list existing ACL roles.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

ACL List Options:

  -json
    Output the ACL roles in a JSON format.

  -t
    Format and display the ACL roles using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (a *ACLRoleListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (a *ACLRoleListCommand) AutocompleteArgs() complete.Predictor { return complete.PredictNothing }

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLRoleListCommand) Synopsis() string { return "List ACL roles" }

// Name returns the name of this command.
func (a *ACLRoleListCommand) Name() string { return "acl role list" }

// Run satisfies the cli.Command Run function.
func (a *ACLRoleListCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

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
	roles, _, err := client.ACLRoles().List(nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error listing ACL roles: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, roles)
		if err != nil {
			a.Ui.Error(err.Error())
			return 1
		}

		a.Ui.Output(out)
		return 0
	}

	a.Ui.Output(formatACLRoles(roles))
	return 0
}

func formatACLRoles(roles []*api.ACLRoleListStub) string {
	if len(roles) == 0 {
		return "No ACL roles found"
	}

	output := make([]string, 0, len(roles)+1)
	output = append(output, "ID|Name|Description|Policies")
	for _, role := range roles {
		output = append(output, fmt.Sprintf(
			"%s|%s|%s|%s",
			role.ID, role.Name, role.Description, strings.Join(aclRolePolicyLinkToStringList(role.Policies), ",")))
	}

	return formatList(output)
}
