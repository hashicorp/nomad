// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLAuthMethodDeleteCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodDeleteCommand{}

// ACLAuthMethodDeleteCommand implements cli.Command.
type ACLAuthMethodDeleteCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (a *ACLAuthMethodDeleteCommand) Help() string {
	helpText := `
Usage: nomad acl auth-method delete <acl_method_name>

  Delete is used to delete an existing ACL auth method. Use requires a
  management token.

General Options:


  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (a *ACLAuthMethodDeleteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (a *ACLAuthMethodDeleteCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLAuthMethodDeleteCommand) Synopsis() string { return "Delete an existing ACL auth method" }

// Name returns the name of this command.
func (a *ACLAuthMethodDeleteCommand) Name() string { return "acl auth-method delete" }

// Run satisfies the cli.Command Run function.
func (a *ACLAuthMethodDeleteCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that the last argument is the auth method name to delete.
	if len(flags.Args()) != 1 {
		a.Ui.Error("This command takes one argument: <acl_auth_method_name>")
		a.Ui.Error(commandErrorText(a))
		return 1
	}

	methodName := flags.Args()[0]

	// Get the HTTP client.
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Delete the specified method
	_, err = client.ACLAuthMethods().Delete(methodName, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error deleting ACL auth method: %s", err))
		return 1
	}

	// Give some feedback to indicate the deletion was successful.
	a.Ui.Output(fmt.Sprintf("ACL auth method %s successfully deleted", methodName))
	return 0
}
