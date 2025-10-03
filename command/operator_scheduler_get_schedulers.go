// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

var _ cli.Command = &OperatorSchedulerGetNumSchedulers{}

type OperatorSchedulerGetNumSchedulers struct {
	Meta

	json bool
	tmpl string
}

func (o *OperatorSchedulerGetNumSchedulers) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(o.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		},
	)
}

func (o *OperatorSchedulerGetNumSchedulers) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (o *OperatorSchedulerGetNumSchedulers) Name() string {
	return "operator scheduler get-num-schedulers"
}

func (o *OperatorSchedulerGetNumSchedulers) Run(args []string) int {

	flags := o.Meta.FlagSet("get-num-schedulers", FlagSetClient)
	flags.BoolVar(&o.json, "json", false, "")
	flags.StringVar(&o.tmpl, "t", "", "")
	flags.Usage = func() { o.Ui.Output(o.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Set up a client.
	client, err := o.Meta.Client()
	if err != nil {
		o.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Fetch the current configuration.
	resp, _, err := client.Operator().SchedulerGetNumSchedulers(&api.QueryOptions{AllowStale: true})
	if err != nil {
		o.Ui.Error(fmt.Sprintf("Error querying number of schedulers: %s", err))
		return 1
	}

	// If the user has specified to output the scheduler config as JSON or
	// using a template, perform this action for the entire object and exit the
	// command.
	if o.json || len(o.tmpl) > 0 {
		out, err := Format(o.json, o.tmpl, resp)
		if err != nil {
			o.Ui.Error(err.Error())
			return 1
		}
		o.Ui.Output(out)
		return 0
	}

	numSched := resp.Schedulers

	// Output the information.
	o.Ui.Output(formatKV([]string{
		fmt.Sprintf("Server address|???"),
		fmt.Sprintf("Number of schedulers|%d", numSched),
	}))
	return 0
}

func (o *OperatorSchedulerGetNumSchedulers) Synopsis() string {
	return "Display the number of schedulers for the server"
}

func (o *OperatorSchedulerGetNumSchedulers) Help() string {
	helpText := `
Usage: nomad operator scheduler get-schedulers [options]

  Displays the number of schedulers per server.

  If ACLs are enabled, this command requires a token with the 'operator:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Scheduler Get Config Options:

  -json
    Output the schedulers in its JSON format.

  -t
    Format and display the schedulers using a Go template.
`

	return strings.TrimSpace(helpText)
}
