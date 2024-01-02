// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type OperatorMetricsCommand struct {
	Meta
}

func (c *OperatorMetricsCommand) Help() string {
	helpText := `
Usage: nomad operator metrics [options]

Get Nomad metrics

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Metrics Specific Options

  -pretty
    Pretty prints the JSON output

  -format <format>
    Specify output format (prometheus)

  -json
    Output the allocation in its JSON format.

  -t
    Format and display allocation using a Go template.

`

	return strings.TrimSpace(helpText)
}

func (c *OperatorMetricsCommand) Synopsis() string {
	return "Retrieve Nomad metrics"
}

func (c *OperatorMetricsCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-pretty": complete.PredictAnything,
			"-format": complete.PredictAnything,
			"-json":   complete.PredictNothing,
			"-t":      complete.PredictAnything,
		})
}

func (c *OperatorMetricsCommand) Name() string { return "metrics" }

func (c *OperatorMetricsCommand) Run(args []string) int {
	var pretty, json bool
	var format, tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&pretty, "pretty", false, "")
	flags.StringVar(&format, "format", "", "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing flags: %s", err))
		return 1
	}

	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	params := map[string]string{}

	if pretty {
		params["pretty"] = "1"
	}

	if len(format) > 0 {
		params["format"] = format
	}

	query := &api.QueryOptions{
		Params: params,
	}

	if json || len(tmpl) > 0 {
		metrics, _, err := client.Operator().MetricsSummary(query)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying metrics: %v", err))
			return 1
		}

		out, err := Format(json, tmpl, metrics)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	} else {
		bs, err := client.Operator().Metrics(query)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error getting metrics: %v", err))
			return 1
		}

		resp := string(bs)
		c.Ui.Output(resp)
	}

	return 0
}
