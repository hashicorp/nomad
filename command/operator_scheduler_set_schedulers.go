// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/cli"
	"github.com/posener/complete"
)

// Ensure OperatorSchedulerSetConfig satisfies the cli.Command interface.
var _ cli.Command = &OperatorSchedulerSetNumSchedulers{}

type OperatorSchedulerSetNumSchedulers struct {
	Meta
}

func (o *OperatorSchedulerSetNumSchedulers) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(o.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
		},
	)
}

func (o *OperatorSchedulerSetNumSchedulers) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (o *OperatorSchedulerSetNumSchedulers) Name() string {
	return "operator scheduler set-num-schedulers"
}

func (o *OperatorSchedulerSetNumSchedulers) Run(args []string) int {

	var jsonInput bool
	flags := o.Meta.FlagSet("set-num-schedulers", FlagSetClient)
	flags.Usage = func() { o.Ui.Output(o.Help()) }
	flags.BoolVar(&jsonInput, "json", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we get exactly one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		o.Ui.Error("This command takes one argument: <number>")
		o.Ui.Error(commandErrorText(o))
		return 1
	}

	number, err := strconv.Atoi(args[0])
	if err != nil {
		o.Ui.Error(fmt.Sprintf("Invalid number of schedulers to set: %s", args[0]))
		return 1
	}

	// Set up a client.
	client, err := o.Meta.Client()
	if err != nil {
		o.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	o.Ui.Info(fmt.Sprintf("Setting number of schedulers to %d...", number))
	_, _, err = client.Operator().SchedulerSetNumSchedulers(number, nil)
	if err != nil {
		o.Ui.Error(fmt.Sprintf("Error setting number of schedulers: %s", err))
		return 1
	}
	o.Ui.Output("Number of schedulers updated")
	return 0
}

func (o *OperatorSchedulerSetNumSchedulers) Synopsis() string {
	return "Modify the current number of schedulers"
}

func (o *OperatorSchedulerSetNumSchedulers) Help() string {
	helpText := `
Usage: nomad operator scheduler set-num-schedulers [options]

  Sets the number of schedulers.

  If ACLs are enabled, this command requires a token with the 'operator:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `
`
	return strings.TrimSpace(helpText)
}
