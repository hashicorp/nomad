// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"os"
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

  -per-page
    The maximum number of workloads to return per page. If not specified, or set to 0, all results are returned.

  -page-token
    Where to start pagination

  -verbose
    Display full output

  -json
    Display output as json

  -p 
    Sort output by priority instead of creation time
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
			"-p":          complete.PredictNothing,
			"-per-page":   complete.PredictAnything,
			"-page-token": complete.PredictAnything,
		})
}

func (c *QueueJobsCommand) AutocompleteArgs() complete.Predictor {
	return JobPredictor(c.Meta.Client)
}

func (c *QueueJobsCommand) Name() string { return "queue status" }

func (c *QueueJobsCommand) Run(args []string) int {
	var verbose, jsonOut, prioritySort bool
	var pageToken string
	var perPage int
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&jsonOut, "json", false, "")
	flags.BoolVar(&prioritySort, "p", false, "")
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
	qo := &api.QueryOptions{
		PerPage:   int32(perPage),
		NextToken: pageToken,
		Params: map[string]string{
			"sort": "default",
		},
	}

	if prioritySort {
		qo.Params["sort"] = "priority"
	}

	// Submit the request
	resp, qm, err := client.BatchJobQueue().Jobs(qo)
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
		c.printDynamicQueue(resp, qm, jsonOut)
	case "unset":
		c.Ui.Output("No batch job queue configured")
	default:
		c.Ui.Error(fmt.Sprintf("Unknown queue type: %s", resp.Type))
		return 255
	}

	return 0
}

func (c *QueueJobsCommand) printDynamicQueue(resp *api.BatchJobQueueJobsResponse, qm *api.QueryMeta, jsonOut bool) int {
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

	if jsonOut {
		if err := c.printDynamicQueueJSON(workloads); err != nil {
			c.Ui.Error("Error unmarshaling json response")
			return 255
		}
	} else {
		c.printDynamicQueueFormatted(workloads)
		if qm.NextToken != "" {
			c.Ui.Output(fmt.Sprintf(`
Results have been paginated. To get the next page run:

%s -page-token %s`, argsWithoutPageToken(os.Args), qm.NextToken))
		}
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
