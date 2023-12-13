// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type AllocChecksCommand struct {
	Meta
}

func (c *AllocChecksCommand) Help() string {
	helpText := `
Usage: nomad alloc checks [options] <allocation>
Alias: nomad checks

  Outputs the latest health check status information for services in the allocation
  using the Nomad service discovery provider.

General Options:

` + generalOptionsUsage(usageOptsDefault) + `

Checks Specific Options:

  -verbose
    Show full information.

  -json
    Output the latest health check status information in a JSON format.

  -t
    Format and display latest health check status information using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *AllocChecksCommand) Synopsis() string {
	return "Outputs service health check status information."
}

func (c *AllocChecksCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-verbose": complete.PredictNothing,
			"-json":    complete.PredictNothing,
			"-t":       complete.PredictAnything,
		})
}

func (c *AllocChecksCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}
		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Allocs, nil)
		if err != nil {
			return nil
		}
		return resp.Matches[contexts.Allocs]
	})
}

func (c *AllocChecksCommand) Name() string {
	return "alloc checks"
}

func (c *AllocChecksCommand) Run(args []string) int {
	var json, verbose bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got only one argument
	args = flags.Args()
	if numArgs := len(args); numArgs < 1 {
		c.Ui.Error("An allocation ID is required")
		c.Ui.Error(commandErrorText(c))
		return 1
	} else if numArgs > 1 {
		c.Ui.Error("This command takes one argument (allocation ID)")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	allocID := args[0]
	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Query the allocation info
	if len(allocID) == 1 {
		c.Ui.Error("Alloc ID must contain at least two characters.")
		return 1
	}

	allocID = sanitizeUUIDPrefix(allocID)
	allocations, _, err := client.Allocations().PrefixList(allocID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
		return 1
	}
	if len(allocations) == 0 {
		c.Ui.Error(fmt.Sprintf("No allocation(s) with prefix or id %q found", allocID))
		return 1
	}
	if len(allocations) > 1 {
		out := formatAllocListStubs(allocations, verbose, length)
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", out))
		return 1
	}

	// prefix lookup matched single allocation (happy path), lookup the checks
	q := &api.QueryOptions{Namespace: allocations[0].Namespace}
	checks, err := client.Allocations().Checks(allocations[0].ID, q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying allocation checks: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, checks)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(fmt.Sprintf("Status of %d Nomad Service Checks", len(checks)))
	c.Ui.Output("")

	pair := func(key, value string) string { return fmt.Sprintf("%s|=|%s", key, value) }
	taskFmt := func(s string) string {
		if s == "" {
			return "(group)"
		}
		return s
	}

	for _, check := range checks {
		list := []string{
			pair("ID", check.ID),
			pair("Name", check.Check),
			pair("Group", check.Group),
			pair("Task", taskFmt(check.Task)),
			pair("Service", check.Service),
			pair("Status", check.Status),
		}
		if check.StatusCode > 0 {
			list = append(list, pair("StatusCode", fmt.Sprintf("%d", check.StatusCode)))
		}
		list = append(list,
			pair("Mode", check.Mode),
			pair("Timestamp", formatTaskTimes(time.Unix(check.Timestamp, 0))),
			pair("Output", check.Output),
		)

		c.Ui.Output(formatList(list))
		c.Ui.Output("")
	}
	return 0
}
