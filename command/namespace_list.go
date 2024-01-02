// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type NamespaceListCommand struct {
	Meta
}

func (c *NamespaceListCommand) Help() string {
	helpText := `
Usage: nomad namespace list [options]

  List is used to list available namespaces.

  If ACLs are enabled, this command requires a management ACL token to view
  all namespaces. A non-management token can be used to list namespaces for
  which it has an associated capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

List Options:

  -json
    Output the namespaces in a JSON format.

  -t
    Format and display the namespaces using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *NamespaceListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *NamespaceListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *NamespaceListCommand) Synopsis() string {
	return "List namespaces"
}

func (c *NamespaceListCommand) Name() string { return "namespace list" }

func (c *NamespaceListCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	namespaces, _, err := client.Namespaces().List(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving namespaces: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, namespaces)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatNamespaces(namespaces))
	return 0
}

func formatNamespaces(namespaces []*api.Namespace) string {
	if len(namespaces) == 0 {
		return "No namespaces found"
	}

	// Sort the output by namespace name
	sort.Slice(namespaces, func(i, j int) bool { return namespaces[i].Name < namespaces[j].Name })

	rows := make([]string, len(namespaces)+1)
	rows[0] = "Name|Description"
	for i, ns := range namespaces {
		rows[i+1] = fmt.Sprintf("%s|%s",
			ns.Name,
			ns.Description)
	}
	return formatList(rows)
}
