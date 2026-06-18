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

type QueueJobsCommand struct {
	Meta
}

func (c *QueueJobsCommand) Help() string {
	helpText := `
Usage: nomad queue status [options]

  View the current status of workloads queued in a batch job queue.

  When ACLs are enabled, this command requires a token with the 'list-jobs' capability. If multiple jobs in the queue are in different namespaces, the output will be filtered to only include jobs in namespaces the token has permissions for.
  
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

func (c *QueueJobsCommand) Synopsis() string {
	return "View the status of a batch job queue"
}

func (c *QueueJobsCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-verbose": complete.PredictNothing,
			"-limit":   complete.PredictNothing,
			"-json":    complete.PredictNothing,
			"-tenants": complete.PredictNothing,
		})
}

func (c *QueueJobsCommand) AutocompleteArgs() complete.Predictor {
	return JobPredictor(c.Meta.Client)
}

func (c *QueueJobsCommand) Name() string { return "queue status" }

func (c *QueueJobsCommand) Run(args []string) int {
	var verbose, jsonOut, tenants bool
	var limit int
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&jsonOut, "json", false, "")
	flags.BoolVar(&tenants, "tenants", false, "")
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

	// Submit the request
	resp, _, err := client.BatchJobQueue().Jobs(qo)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error during batch queue request: %s", err))
		return 255
	}
	if resp == nil {
		c.Ui.Error("Empty batch queue response")
		return 255
	}

	switch resp.Type {
	case api.BatchJobQueueTypeDynamic:
		return c.printDynamicQueue(resp, jsonOut)
	case "unset":
		c.Ui.Output("No batch job queue configured")
	default:
		c.Ui.Error(fmt.Sprintf("Unknown queue type: %s", resp.Type))
		return 255
	}

	return 0
}

func (c *QueueJobsCommand) printDynamicQueue(resp *api.BatchJobQueueJobsResponse, jsonOut bool) int {
	workloads := []api.DynamicPriorityWorkload{}
	bytes, err := json.Marshal(resp.Workloads)
	if err != nil {
		c.Ui.Error("Error marshaling response status")
		return 255
	}
	if err := json.Unmarshal(bytes, &workloads); err != nil {
		c.Ui.Error("Invalid Status response from server")
		return 255
	}

	slices.SortFunc(workloads, func(a api.DynamicPriorityWorkload, b api.DynamicPriorityWorkload) int {
		if a.AdjustedPriority < b.AdjustedPriority {
			return 1
		} else if b.AdjustedPriority < a.AdjustedPriority {
			return -1
		}
		return 0
	})

	if jsonOut {
		if err := c.printDynamicQueueJSON(workloads); err != nil {
			c.Ui.Error("Error unmarshaling json response")
			return 255
		}
	} else {
		c.printDynamicQueueFormatted(workloads)
	}
	return 0
}

func (c *QueueJobsCommand) printDynamicQueueJSON(resp []api.DynamicPriorityWorkload) error {
	out, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	c.Ui.Output(string(out))
	return nil
}

func (c *QueueJobsCommand) printDynamicQueueFormatted(resp []api.DynamicPriorityWorkload) {
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
			v.UsageAdjustment,
			v.AgeAdjustment,
			v.SizeAdjustment,
		)
	}

	c.Ui.Output(c.Colorize().Color("[bold]Batch Queue Workloads[reset]"))
	c.Ui.Output(columnize.SimpleFormat(out))
}
