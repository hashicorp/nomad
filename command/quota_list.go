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

type QuotaListCommand struct {
	Meta
}

func (c *QuotaListCommand) Help() string {
	helpText := `
Usage: nomad quota list [options]

  List is used to list available quota specifications.

  If ACLs are enabled, this command requires a token with the 'quota:read'
  capability. Any quotas applied to namespaces that the token does not have
  access to will be filtered from the results.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

List Options:

  -json
    Output the quota specifications in a JSON format.

  -t
    Format and display the quota specifications using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *QuotaListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *QuotaListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *QuotaListCommand) Synopsis() string {
	return "List quota specifications"
}

func (c *QuotaListCommand) Name() string { return "quota list" }
func (c *QuotaListCommand) Run(args []string) int {
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

	quotas, _, err := client.Quotas().List(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving quotas: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, quotas)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatQuotaSpecs(quotas))
	return 0
}

func formatQuotaSpecs(quotas []*api.QuotaSpec) string {
	if len(quotas) == 0 {
		return "No quotas found"
	}

	// Sort the output by quota name
	sort.Slice(quotas, func(i, j int) bool { return quotas[i].Name < quotas[j].Name })

	rows := make([]string, len(quotas)+1)
	rows[0] = "Name|Description"
	for i, qs := range quotas {
		rows[i+1] = fmt.Sprintf("%s|%s",
			qs.Name,
			qs.Description)
	}
	return formatList(rows)
}
