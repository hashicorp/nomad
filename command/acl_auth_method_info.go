// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLAuthMethodInfoCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodInfoCommand{}

// ACLAuthMethodInfoCommand implements cli.Command.
type ACLAuthMethodInfoCommand struct {
	Meta

	json bool
	tmpl string
}

// Help satisfies the cli.Command Help function.
func (a *ACLAuthMethodInfoCommand) Help() string {
	helpText := `
Usage: nomad acl auth-method info [options] <acl_method_name>

  Info is used to fetch information on an existing ACL auth method. Requires a
  management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

ACL Info Options:

  -json
    Output the ACL auth method in a JSON format.

  -t
    Format and display the ACL auth method using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (a *ACLAuthMethodInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (a *ACLAuthMethodInfoCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLAuthMethodInfoCommand) Synopsis() string {
	return "Fetch information on an existing ACL auth method"
}

// Name returns the name of this command.
func (a *ACLAuthMethodInfoCommand) Name() string { return "acl auth-method info" }

// Run satisfies the cli.Command Run function.
func (a *ACLAuthMethodInfoCommand) Run(args []string) int {
	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.BoolVar(&a.json, "json", false, "")
	flags.StringVar(&a.tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we have exactly one argument.
	if len(flags.Args()) != 1 {
		a.Ui.Error("This command takes one argument: <acl_auth_method_name>")
		a.Ui.Error(commandErrorText(a))
		return 1
	}

	// Get the HTTP client.
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	methodName := flags.Args()[0]

	method, _, apiErr := client.ACLAuthMethods().Get(methodName, nil)

	// Handle any error from the API.
	if apiErr != nil {
		a.Ui.Error(fmt.Sprintf("Error reading ACL auth method: %s", apiErr))
		return 1
	}

	if a.json || len(a.tmpl) > 0 {
		out, err := Format(a.json, a.tmpl, method)
		if err != nil {
			a.Ui.Error(err.Error())
			return 1
		}

		a.Ui.Output(out)
		return 0
	}

	// Format the output.
	outputAuthMethod(a.Meta, method)
	return 0
}
