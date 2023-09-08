// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type NamespaceInspectCommand struct {
	Meta
}

func (c *NamespaceInspectCommand) Help() string {
	helpText := `
Usage: nomad namespace inspect [options] <namespace>

  Inspect is used to view raw information about a particular namespace.

  If ACLs are enabled, this command requires a management ACL token or a token
  that has a capability associated with the namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Inspect Options:

  -t
    Format and display the namespaces using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (c *NamespaceInspectCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-t": complete.PredictAnything,
		})
}

func (c *NamespaceInspectCommand) AutocompleteArgs() complete.Predictor {
	return NamespacePredictor(c.Meta.Client, nil)
}

func (c *NamespaceInspectCommand) Synopsis() string {
	return "Inspect a namespace"
}

func (c *NamespaceInspectCommand) Name() string { return "namespace inspect" }

func (c *NamespaceInspectCommand) Run(args []string) int {
	var tmpl string
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one arguments
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <namespace>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	name := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Do a prefix lookup
	ns, possible, err := getNamespace(client.Namespaces(), name)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving namespaces: %s", err))
		return 1
	}

	if len(possible) != 0 {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple namespaces\n\n%s", formatNamespaces(possible)))
		return 1
	}

	out, err := Format(len(tmpl) == 0, tmpl, ns)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	c.Ui.Output(out)
	return 0
}
