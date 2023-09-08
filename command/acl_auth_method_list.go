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

// Ensure ACLAuthMethodListCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodListCommand{}

// ACLAuthMethodListCommand implements cli.Command.
type ACLAuthMethodListCommand struct {
	Meta

	json bool
	tmpl string
}

// Help satisfies the cli.Command Help function.
func (a *ACLAuthMethodListCommand) Help() string {
	helpText := `
Usage: nomad acl auth-method list [options]

  List is used to list existing ACL auth methods.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

ACL List Options:

  -json
    Output the ACL auth methods in a JSON format.

  -t
    Format and display the ACL auth methods using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (a *ACLAuthMethodListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (a *ACLAuthMethodListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLAuthMethodListCommand) Synopsis() string { return "List ACL auth methods" }

// Name returns the name of this command.
func (a *ACLAuthMethodListCommand) Name() string { return "acl auth-method list" }

// Run satisfies the cli.Command Run function.
func (a *ACLAuthMethodListCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.BoolVar(&a.json, "json", false, "")
	flags.StringVar(&a.tmpl, "t", "", "")

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

	// Fetch info on the method
	methods, _, err := client.ACLAuthMethods().List(nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error listing ACL auth methods: %s", err))
		return 1
	}

	if a.json || len(a.tmpl) > 0 {
		out, err := Format(a.json, a.tmpl, methods)
		if err != nil {
			a.Ui.Error(err.Error())
			return 1
		}

		a.Ui.Output(out)
		return 0
	}

	a.Ui.Output(formatAuthMethods(methods))
	return 0
}

func formatAuthMethods(methods []*api.ACLAuthMethodListStub) string {
	if len(methods) == 0 {
		return "No ACL auth methods found"
	}

	output := make([]string, 0, len(methods)+1)
	output = append(output, "Name|Type|Default")
	for _, method := range methods {
		output = append(output, fmt.Sprintf(
			"%s|%s|%v",
			method.Name, method.Type, method.Default))
	}

	return formatList(output)
}
