// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type OperatorUtilizationCommand struct {
	Meta
}

func (c *OperatorUtilizationCommand) Help() string {
	helpText := `
Usage: nomad operator utilization [options]

  This command allows Nomad Enterprise users to generate utilization reporting
  bundles. If you have disabled automated reporting, use this command to
  manually generate the report and send it to HashiCorp. If no snapshots were
  persisted in the last 24 hrs, Nomad takes a new snapshot.

  If ACLs are enabled, this command requires a token with the 'operator:write'
  capability.

  -message
    Provide context about the conditions under which the report was generated
    and submitted. This message is not included in the utilization bundle but
    will be included in the Nomad server logs.

  -output
    Specifies the output path for the bundle. Defaults to a time-based generated
    file name in the current working directory.

  -today-only
    Include snapshots from the previous 24 hours, not historical snapshots.

` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *OperatorUtilizationCommand) Synopsis() string {
	return "Generate a utilization reporting bundle"
}

func (c *OperatorUtilizationCommand) Name() string { return "operator utilization" }

func (c *OperatorUtilizationCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-message":    complete.PredictNothing,
			"-today-only": complete.PredictNothing,
			"-output":     complete.PredictFiles(""),
		})
}

func (c *OperatorUtilizationCommand) Run(args []string) int {
	var todayOnly bool
	var message, outputPath string

	flags := c.Meta.FlagSet("operator utilization", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&todayOnly, "today-only", false, "only today's snapshot")
	flags.StringVar(&outputPath, "output", "", "output path for the bundle")
	flags.StringVar(&message, "message", "", "provided context for logs")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if len(args) != 0 {
		c.Ui.Error("This command requires no arguments.")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating nomad API client: %s", err))
		return 1
	}

	resp, _, err := client.Operator().Utilization(
		&api.OperatorUtilizationOptions{TodayOnly: todayOnly}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error generating bundle: %s", err))
		return 1
	}

	if outputPath == "" {
		t := time.Now().Unix()
		outputPath = fmt.Sprintf("nomad-utilization-%v.json", t)
	}

	err = os.WriteFile(outputPath, resp.Bundle, 0600)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Could not write bundle to file: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf(
		"Success! Utilization reporting bundle written to: %s", outputPath))
	return 0
}
