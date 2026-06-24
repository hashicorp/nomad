// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
	"github.com/ryanuber/columnize"
)

type QueueJobsCommand struct {
	sortOrder string
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

  -per-page
    The maximum number of workloads to return per page. If not specified, or set to 0, all results are returned.

  -page-token
    Where to start pagination

  -verbose
    Display full output

  -json
    Display output as json

  -sort <>
    Sort the output by a specific field. Valid fields are: priority, created_at. Default is created_at.
`
	return strings.TrimSpace(helpText)
}

func (c *QueueJobsCommand) Synopsis() string {
	return "View the status of a batch job queue"
}

func (c *QueueJobsCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-verbose":    complete.PredictNothing,
			"-json":       complete.PredictNothing,
			"-sort":       complete.PredictAnything,
			"-per-page":   complete.PredictAnything,
			"-page-token": complete.PredictAnything,
		})
}

func (c *QueueJobsCommand) AutocompleteArgs() complete.Predictor {
	return JobPredictor(c.Meta.Client)
}

func (c *QueueJobsCommand) Name() string { return "queue status" }

func (c *QueueJobsCommand) Run(args []string) int {
	var verbose, jsonOut bool
	var pageToken string
	var perPage int
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&jsonOut, "json", false, "")
	flags.StringVar(&c.sortOrder, "sort", "created_at", "")
	flags.StringVar(&pageToken, "page-token", "", "")
	flags.IntVar(&perPage, "per-page", 0, "")

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

	qo.PerPage = int32(perPage)
	qo.NextToken = pageToken

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

	slices.SortFunc(workloads, func(a, b api.DynamicPriorityWorkload) int {
		if c.sortOrder == "priority" || a.CreatedAt == b.CreatedAt {
			return cmp.Compare(a.Position, b.Position)
		}
		return cmp.Compare(a.CreatedAt, b.CreatedAt)
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
	out[0] = "JobID|Tenant|Adjusted Priority|Base Priority|Position|Usage|Age|Size|CreatedAt"

	for i, v := range resp {
		out[i+1] = fmt.Sprintf("%s|%s|%d|%d|%d|%d|%d|%d|%s",
			v.JobID,
			v.Tenant,
			v.AdjustedPriority,
			v.BasePriority,
			v.Position,
			v.UsageAdjustment,
			v.AgeAdjustment,
			v.SizeAdjustment,
			formatUnixNanoTime(v.CreatedAt),
		)
	}

	c.Ui.Output(c.Colorize().Color("[bold]Batch Queue Workloads[reset]"))
	c.Ui.Output(columnize.SimpleFormat(out))
}
