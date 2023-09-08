// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/posener/complete"
)

type NodeMetaReadCommand struct {
	Meta
}

func (c *NodeMetaReadCommand) Help() string {
	helpText := `
Usage: nomad node meta read [-json] [-node-id ...]

  Read a node's metadata. This command only works on client agents. The node
  status command can be used to retrieve node metadata from any agent.

  Changes via the "node meta apply" subcommand are batched and may take up to
  10 seconds to propagate to the servers and affect scheduling. This command
  will always return the most recent node metadata while the "node status"
  command can be used to view the metadata that is currently being used for
  scheduling.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Node Meta Options:

  -node-id
    Reads metadata from the specified node. If not specified the node receiving
    the request will be used by default.

  -json
    Output the node metadata in its JSON format.

  -t
    Format and display node metadata using a Go template.

    Example:
      $ nomad node meta read -node-id 3b58b0a6
`
	return strings.TrimSpace(helpText)
}

func (c *NodeMetaReadCommand) Synopsis() string {
	return "Read node metadata"
}

func (c *NodeMetaReadCommand) Name() string { return "node meta read" }

func (c *NodeMetaReadCommand) Run(args []string) int {
	var nodeID, tmpl string
	var json bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&nodeID, "node-id", "", "")
	flags.StringVar(&tmpl, "t", "", "")
	flags.BoolVar(&json, "json", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Lookup nodeID
	if nodeID != "" {
		nodeID, err = lookupNodeID(client.Nodes(), nodeID)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	}

	meta, err := client.Nodes().Meta().Read(nodeID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error reading dynamic node metadata: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, meta)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(c.Colorize().Color("[bold]All Meta[reset]"))
	c.Ui.Output(formatNodeMeta(meta.Meta))

	// Print dynamic meta
	c.Ui.Output(c.Colorize().Color("\n[bold]Dynamic Meta[reset]"))
	keys := make([]string, 0, len(meta.Dynamic))
	for k := range meta.Dynamic {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var rows []string
	for _, k := range keys {
		v := "<unset>"
		if meta.Dynamic[k] != nil {
			v = *meta.Dynamic[k]
		}
		rows = append(rows, fmt.Sprintf("%s|%s", k, v))
	}
	c.Ui.Output(formatKV(rows))

	// Print static meta
	c.Ui.Output(c.Colorize().Color("\n[bold]Static Meta[reset]"))
	c.Ui.Output(formatNodeMeta(meta.Static))

	return 0
}

func (c *NodeMetaReadCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-node-id": complete.PredictAnything,
			"-json":    complete.PredictNothing,
			"-t":       complete.PredictAnything,
		})
}

func (c *NodeMetaReadCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}
