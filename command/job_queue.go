// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"slices"
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
	var verbose, jsonOut bool
	var limit int
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&jsonOut, "json", false, "")
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
		qo.PerPage = int32(limit)
	}
	qo.Region = "global"

	// Submit the request
	resp, _, err := client.Jobs().BatchQueueStatus(nil, qo)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error during batch queue request: %s", err))
		return 255
	}
	if resp == nil {
		c.Ui.Error("Empty batch queue response")
	}

	switch resp.Type {
	case api.BatchQueueTypeDynamic:
		status := api.DynamicPriorityStatus{}
		bytes, err := json.Marshal(resp.Status)
		if err != nil {
			c.Ui.Error("Error marshaling response status")
			return 255
		}
		if err := json.Unmarshal(bytes, &status); err != nil {
			c.Ui.Error("Invalid Status response from server")
			return 255
		}

		slices.SortFunc(status, func(a api.DynamicPriorityWorkload, b api.DynamicPriorityWorkload) int {
			if a.AdjustedPriority < b.AdjustedPriority {
				return 1
			} else if b.AdjustedPriority < a.AdjustedPriority {
				return -1
			}
			return 0
		})

		if jsonOut {
			if err := c.printDynamicQueueJSON(status); err != nil {
				c.Ui.Error("Error unmarshaling json response")
				return 255
			}
		} else {
			c.printDynamicQueueFormatted(status)
		}
	default:
		c.Ui.Error("Unknown queue type")
		return 255
	}

	return 0
}

func (c *JobQueueCommand) printDynamicQueueJSON(resp api.DynamicPriorityStatus) error {
	out, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	c.Ui.Output(string(out))
	return nil
}

func (c *JobQueueCommand) printDynamicQueueFormatted(resp api.DynamicPriorityStatus) {
	if resp == nil {
		return
	}

	out := make([]string, len(resp)+1)
	out[0] = "JobID|Tenant|Adjusted Priority|Base Priority|Usage|Age|Size"

	for i, v := range resp {
		out[i+1] = fmt.Sprintf("%s|%s|%d|%d|%d|%d|%d",
			v.JobID,
			v.Tenant,
			v.AdjustedPriority,
			v.BasePriority,
			v.UsageAjustment,
			v.AgeAdjustment,
			v.SizeAdjustment,
		)
	}

	c.Ui.Output(c.Colorize().Color("[bold]Batch Queue Workloads[reset]"))
	c.Ui.Output(columnize.SimpleFormat(out))
}
