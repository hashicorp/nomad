// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
	"github.com/ryanuber/columnize"
)

type JobQueueCommand struct {
	Meta
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

  -json
    Display output as json

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
			"-json":    complete.PredictNothing,
		})
}

func (c *JobQueueCommand) AutocompleteArgs() complete.Predictor {
	return JobPredictor(c.Meta.Client)
}

func (c *JobQueueCommand) Name() string { return "job queue" }

func (c *JobQueueCommand) Run(args []string) int {
	var verbose, json bool
	var limit int
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&json, "json", false, "")
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
	qo := &api.QueryOptions{}

	if limit > 0 {
		qo.Params["limit"] = fmt.Sprintf("%d", limit)
	}

	// Submit the request
	resp, _, err := client.Jobs().BatchQueueStatus(nil, qo)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error during batch queue request: %s", err))
		return 255
	}
	if resp == nil {
		c.Ui.Error("Empty batch queue response")
	}

	if json {
		if err := c.printJSON(resp); err != nil {
			c.Ui.Error("Error unmarshaling json response")
			return 255
		}
	} else {
		c.printFormatted(resp)
	}
	return 0
}

func (c *JobQueueCommand) printJSON(resp *api.BatchQueueStatusResponse) error {
	out, err := json.Marshal(resp.Workloads)
	if err != nil {
		return err
	}

	c.Ui.Output(string(out))
	return nil
}

func (c *JobQueueCommand) printFormatted(resp *api.BatchQueueStatusResponse) {

	out := make([]string, len(resp.Workloads)+1)
	out[0] = "JobID|Tenant|Priority"

	for i, v := range resp.Workloads {
		out[i+1] = fmt.Sprintf("%s|%s|%d", v.JobID, v.Tenant, v.Priority)
	}

	c.Ui.Output(c.Colorize().Color("[bold]Batch Queue Workloads[reset]"))
	c.Ui.Output(columnize.SimpleFormat(out))
}
