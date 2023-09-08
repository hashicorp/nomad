// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type NodePoolListCommand struct {
	Meta
}

func (c *NodePoolListCommand) Name() string {
	return "node pool list"
}

func (c *NodePoolListCommand) Synopsis() string {
	return "List node pools"
}

func (c *NodePoolListCommand) Help() string {
	helpText := `
Usage: nomad node pool list [options]

  List is used to list existing node pools.

  If ACLs are enabled, this command requires a management token to view all
  node pools. A non-management token can be used to list node pools for which
  the token has the 'read' capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

List Options:

  -filter
    Specifies an expression used to filter results.

  -json
    Output the node pools in JSON format.

  -page-token
    Where to start pagination.

  -per-page
    How many results to show per page. If not specified, or set to 0, all
    results are returned.

  -t
    Format and display the node pools using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *NodePoolListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-filter":     complete.PredictAnything,
			"-json":       complete.PredictNothing,
			"-page-token": complete.PredictAnything,
			"-per-page":   complete.PredictAnything,
			"-t":          complete.PredictAnything,
		})
}

func (c *NodePoolListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *NodePoolListCommand) Run(args []string) int {
	var json bool
	var perPage int
	var tmpl, pageToken, filter string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&filter, "filter", "", "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&pageToken, "page-token", "", "")
	flags.IntVar(&perPage, "per-page", 0, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we don't have any arguments.
	if len(flags.Args()) != 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Make list request.
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	opts := &api.QueryOptions{
		Filter:    filter,
		PerPage:   int32(perPage),
		NextToken: pageToken,
	}
	pools, qm, err := client.NodePools().List(opts)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying node pools: %s", err))
		return 1
	}

	// Format output if requested.
	if json || tmpl != "" {
		out, err := Format(json, tmpl, pools)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error formatting output: %s", err))
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatNodePoolList(pools))

	if qm.NextToken != "" {
		c.Ui.Output(fmt.Sprintf(`
Results have been paginated. To get the next page run:

%s -page-token %s`, argsWithoutPageToken(os.Args), qm.NextToken))
	}

	return 0
}
