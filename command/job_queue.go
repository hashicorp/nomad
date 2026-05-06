// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type JobQueueCommand struct {
	Meta
	forceRescheduling bool
}

func (c *JobQueueCommand) Help() string {
	helpText := `
Usage: nomad job queue [options]

  View the current status of workloads queued in a batch job queue.

  When ACLs are enabled, this command requires a token with either TBD
  capabilities. Probably at least 'list-jobs'.
  
General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Eval Options:

  -limit
    The maximum number of workloads to return

  -verbose
    Display full output

`
	return strings.TrimSpace(helpText)
}

func (c *JobQueueCommand) Synopsis() string {
	return "View the status of a batch job queue"
}

func (c *JobQueueCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-verbose": complete.PredictNothing,
			"-limit":   complete.PredictNothing,
		})
}

func (c *JobQueueCommand) AutocompleteArgs() complete.Predictor {
	return JobPredictor(c.Meta.Client)
}

func (c *JobQueueCommand) Name() string { return "job queue" }

func (c *JobQueueCommand) Run(args []string) int {
	var verbose bool
	var limit int
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.IntVar(&limit, "limit", 0, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 255
	}

	// Setup the options
	opts := &api.QueryOptions{}

	if limit > 0 {
		opts.Params["limit"] = fmt.Sprintf("%d", limit)
	}

	// Submit the request
	resp, _, err := client.Jobs().BatchQueueStatus(opts)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error during batch queue request: %s", err))
		return 255
	}

	c.printOutput(resp)
	return 0
}

func (c *JobQueueCommand) printOutput(resp *api.BatchQueueStatusResponse) {
	if resp == nil {
		c.Ui.Error("Empty batch queue response")
	}

	headers := []string{"JobID", "Tenant", "Priority"}

	// compute column widths
	col0, col1, col2 := len(headers[0]), len(headers[1]), len(headers[2])
	for _, r := range resp.Workloads {
		col0 = max(col0, len(r.JobID))
		col1 = max(col1, len(r.Tenant))
		col2 = max(col2, len(fmt.Sprintf("%d", r.Priority)))
	}

	headerFmt := fmt.Sprintf("%%-%ds | %%-%ds | %%-%ds\n", col0, col1, col2)
	rowFmt := fmt.Sprintf("%%-%ds | %%-%ds | %%%dd\n", col0, col1, col2)

	var output strings.Builder

	// print header
	fmt.Fprintf(&output, headerFmt, headers[0], headers[1], headers[2])

	// print separator
	fmt.Fprintf(&output, "%s-+-%s-+-%s\n",
		strings.Repeat("-", col0),
		strings.Repeat("-", col1),
		strings.Repeat("-", col2),
	)

	// print rows
	for _, w := range resp.Workloads {
		fmt.Fprintf(&output, rowFmt, w.JobID, w.Tenant, w.Priority)
	}

	c.Ui.Output(output.String())
}
