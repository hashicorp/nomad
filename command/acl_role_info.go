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

// Ensure ACLRoleInfoCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLRoleInfoCommand{}

// ACLRoleInfoCommand implements cli.Command.
type ACLRoleInfoCommand struct {
	Meta

	byName bool
	json   bool
	tmpl   string
}

// Help satisfies the cli.Command Help function.
func (a *ACLRoleInfoCommand) Help() string {
	helpText := `
Usage: nomad acl role info [options] <acl_role_id>

  Info is used to fetch information on an existing ACL roles. Requires a
  management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

ACL Info Options:

  -by-name
    Look up the ACL role using its name as the identifier. The command defaults
    to expecting the ACL ID as the argument.

  -json
    Output the ACL role in a JSON format.

  -t
    Format and display the ACL role using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (a *ACLRoleInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-by-name": complete.PredictNothing,
			"-json":    complete.PredictNothing,
			"-t":       complete.PredictAnything,
		})
}

func (a *ACLRoleInfoCommand) AutocompleteArgs() complete.Predictor { return complete.PredictNothing }

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLRoleInfoCommand) Synopsis() string { return "Fetch information on an existing ACL role" }

// Name returns the name of this command.
func (a *ACLRoleInfoCommand) Name() string { return "acl role info" }

// Run satisfies the cli.Command Run function.
func (a *ACLRoleInfoCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.BoolVar(&a.byName, "by-name", false, "")
	flags.BoolVar(&a.json, "json", false, "")
	flags.StringVar(&a.tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we have exactly one argument.
	if len(flags.Args()) != 1 {
		a.Ui.Error("This command takes one argument: <acl_role_id>")
		a.Ui.Error(commandErrorText(a))
		return 1
	}

	// Get the HTTP client.
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	var (
		aclRole *api.ACLRole
		apiErr  error
	)

	aclRoleID := flags.Args()[0]

	// Use the correct API call depending on whether the lookup is by the name
	// or the ID.
	switch a.byName {
	case true:
		aclRole, _, apiErr = client.ACLRoles().GetByName(aclRoleID, nil)
	default:
		aclRole, _, apiErr = client.ACLRoles().Get(aclRoleID, nil)
	}

	// Handle any error from the API.
	if apiErr != nil {
		a.Ui.Error(fmt.Sprintf("Error reading ACL role: %s", apiErr))
		return 1
	}

	// Format the output.
	a.Ui.Output(formatACLRole(aclRole))
	return 0
}
