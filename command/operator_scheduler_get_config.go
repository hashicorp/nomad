// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure OperatorSchedulerGetConfig satisfies the cli.Command interface.
var _ cli.Command = &OperatorSchedulerGetConfig{}

type OperatorSchedulerGetConfig struct {
	Meta

	json bool
	tmpl string
}

func (o *OperatorSchedulerGetConfig) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(o.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		},
	)
}

func (o *OperatorSchedulerGetConfig) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (o *OperatorSchedulerGetConfig) Name() string { return "operator scheduler get-config" }

func (o *OperatorSchedulerGetConfig) Run(args []string) int {

	flags := o.Meta.FlagSet("get-config", FlagSetClient)
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
	resp, _, err := client.Operator().SchedulerGetConfiguration(nil)
	if err != nil {
		o.Ui.Error(fmt.Sprintf("Error querying scheduler configuration: %s", err))
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

	schedConfig := resp.SchedulerConfig

	// Output the information.
	o.Ui.Output(formatKV([]string{
		fmt.Sprintf("Scheduler Algorithm|%s", schedConfig.SchedulerAlgorithm),
		fmt.Sprintf("Memory Oversubscription|%v", schedConfig.MemoryOversubscriptionEnabled),
		fmt.Sprintf("Reject Job Registration|%v", schedConfig.RejectJobRegistration),
		fmt.Sprintf("Pause Eval Broker|%v", schedConfig.PauseEvalBroker),
		fmt.Sprintf("Preemption System Scheduler|%v", schedConfig.PreemptionConfig.SystemSchedulerEnabled),
		fmt.Sprintf("Preemption Service Scheduler|%v", schedConfig.PreemptionConfig.ServiceSchedulerEnabled),
		fmt.Sprintf("Preemption Batch Scheduler|%v", schedConfig.PreemptionConfig.BatchSchedulerEnabled),
		fmt.Sprintf("Preemption SysBatch Scheduler|%v", schedConfig.PreemptionConfig.SysBatchSchedulerEnabled),
		fmt.Sprintf("Modify Index|%v", resp.SchedulerConfig.ModifyIndex),
	}))
	return 0
}

func (o *OperatorSchedulerGetConfig) Synopsis() string {
	return "Display the current scheduler configuration"
}

func (o *OperatorSchedulerGetConfig) Help() string {
	helpText := `
Usage: nomad operator scheduler get-config [options]

  Displays the current scheduler configuration.

  If ACLs are enabled, this command requires a token with the 'operator:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Scheduler Get Config Options:

  -json
    Output the scheduler config in its JSON format.

  -t
    Format and display the scheduler config using a Go template.
`

	return strings.TrimSpace(helpText)
}
