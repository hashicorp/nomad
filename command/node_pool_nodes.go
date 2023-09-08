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

type NodePoolNodesCommand struct {
	Meta
}

func (c *NodePoolNodesCommand) Name() string {
	return "node pool nodes"
}

func (c *NodePoolNodesCommand) Synopsis() string {
	return "Fetch a list of nodes in a node pool"
}

func (c *NodePoolNodesCommand) Help() string {
	helpText := `
Usage: nomad node pool nodes <node-pool>

  Node pool nodes is used to list nodes in a given node pool.

  If ACLs are enabled, this command requires a token with the 'node:read'
  capability and the 'read' capability in a 'node_pool' policy that matches the
  node pool being targeted.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Node Pool Nodes Options:

  -filter
    Specifies an expression used to filter nodes from the results. The filter
    is applied to the nodes and not the node pool.

  -json
    Output the list in JSON format.

  -page-token
    Where to start pagination.

  -per-page
    How many results to show per page. If not specified, or set to 0, all
    results are returned.

  -t
    Format and display nodes list using a Go template.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *NodePoolNodesCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-filter":     complete.PredictAnything,
			"-json":       complete.PredictNothing,
			"-page-token": complete.PredictAnything,
			"-per-page":   complete.PredictAnything,
			"-t":          complete.PredictAnything,
			"-verbose":    complete.PredictNothing,
		})
}

func (c *NodePoolNodesCommand) AutocompleteArgs() complete.Predictor {
	return nodePoolPredictor(c.Client, nil)
}

func (c *NodePoolNodesCommand) Run(args []string) int {
	var json, verbose bool
	var perPage int
	var pageToken, filter, tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&filter, "filter", "", "")
	flags.StringVar(&pageToken, "page-token", "", "")
	flags.IntVar(&perPage, "per-page", 0, "")
	flags.StringVar(&tmpl, "t", "", "")
	flags.BoolVar(&verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we only have one argument.
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <node-pool>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Lookup node pool by prefix.
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	pool, possible, err := nodePoolByPrefix(client, args[0])
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving node pool: %s", err))
		return 1
	}
	if len(possible) != 0 {
		c.Ui.Error(fmt.Sprintf(
			"Prefix matched multiple node pools\n\n%s", formatNodePoolList(possible)))
		return 1
	}

	opts := &api.QueryOptions{
		Filter:    filter,
		PerPage:   int32(perPage),
		NextToken: pageToken,
	}
	nodes, qm, err := client.NodePools().ListNodes(pool.Name, opts)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying nodes: %s", err))
		return 1
	}

	if len(nodes) == 0 {
		c.Ui.Output("No nodes")
		return 0
	}

	// Format output if requested.
	if json || tmpl != "" {
		out, err := Format(json, tmpl, nodes)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatNodeStubList(nodes, verbose))

	if qm.NextToken != "" {
		c.Ui.Output(fmt.Sprintf(`
Results have been paginated. To get the next page run:

%s -page-token %s`, argsWithoutPageToken(os.Args), qm.NextToken))
	}

	return 0
}
