// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLRoleDeleteCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLRoleDeleteCommand{}

// ACLRoleDeleteCommand implements cli.Command.
type ACLRoleDeleteCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (a *ACLRoleDeleteCommand) Help() string {
	helpText := `
Usage: nomad acl role delete <acl_role_id>

  Delete is used to delete an existing ACL role. Use requires a management
  token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (a *ACLRoleDeleteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (a *ACLRoleDeleteCommand) AutocompleteArgs() complete.Predictor { return complete.PredictNothing }

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLRoleDeleteCommand) Synopsis() string { return "Delete an existing ACL role" }

// Name returns the name of this command.
func (a *ACLRoleDeleteCommand) Name() string { return "acl role delete" }

// Run satisfies the cli.Command Run function.
func (a *ACLRoleDeleteCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that the last argument is the role ID to delete.
	if len(flags.Args()) != 1 {
		a.Ui.Error("This command takes one argument: <acl_role_id>")
		a.Ui.Error(commandErrorText(a))
		return 1
	}

	aclRoleID := flags.Args()[0]

	// Get the HTTP client.
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Delete the specified ACL role.
	_, err = client.ACLRoles().Delete(aclRoleID, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error deleting ACL role: %s", err))
		return 1
	}

	// Give some feedback to indicate the deletion was successful.
	a.Ui.Output(fmt.Sprintf("ACL role %s successfully deleted", aclRoleID))
	return 0
}
