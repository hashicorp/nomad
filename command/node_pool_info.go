// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/posener/complete"
)

type NodePoolInfoCommand struct {
	Meta
}

func (c *NodePoolInfoCommand) Name() string {
	return "node pool info"
}

func (c *NodePoolInfoCommand) Synopsis() string {
	return "Fetch information about an existing node pool"
}

func (c *NodePoolInfoCommand) Help() string {
	helpText := `
Usage: nomad node pool info <node-pool>

  Info is used to fetch information about an existing node pool.

  If ACLs are enabled, this command requires a token with the 'read'
  capability in a 'node_pool' policy that matches the node pool being targeted.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Info Options:

  -json
    Output the node pool in its JSON format.

  -t
    Format and display node pool using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (c *NodePoolInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *NodePoolInfoCommand) AutocompleteArgs() complete.Predictor {
	return nodePoolPredictor(c.Client, nil)
}

func (c *NodePoolInfoCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

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
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple node pools\n\n%s", formatNodePoolList(possible)))
		return 1
	}

	// Format output if requested.
	if json || tmpl != "" {
		out, err := Format(json, tmpl, pool)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// Print node pool information.
	basic := []string{
		fmt.Sprintf("Name|%s", pool.Name),
		fmt.Sprintf("Description|%s", pool.Description),
	}
	c.Ui.Output(formatKV(basic))

	c.Ui.Output(c.Colorize().Color("\n[bold]Metadata[reset]"))
	if len(pool.Meta) > 0 {
		var meta []string
		for k, v := range pool.Meta {
			meta = append(meta, fmt.Sprintf("%s|%s", k, v))
		}
		sort.Strings(meta)
		c.Ui.Output(formatKV(meta))
	} else {
		c.Ui.Output("No metadata")
	}

	c.Ui.Output(c.Colorize().Color("\n[bold]Scheduler Configuration[reset]"))
	if schedConfig := pool.SchedulerConfiguration; schedConfig != nil {
		schedConfigOut := []string{
			fmt.Sprintf("Scheduler Algorithm|%s", schedConfig.SchedulerAlgorithm),
		}
		if schedConfig.MemoryOversubscriptionEnabled != nil {
			schedConfigOut = append(schedConfigOut,
				fmt.Sprintf("Memory Oversubscription Enabled|%v", *schedConfig.MemoryOversubscriptionEnabled),
			)
		}
		c.Ui.Output(formatKV(schedConfigOut))
	} else {
		c.Ui.Output("No scheduler configuration")
	}

	return 0
}
