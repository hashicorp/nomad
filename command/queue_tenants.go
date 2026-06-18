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

type QueueTenantsCommand struct {
	Meta
}

func (c *QueueTenantsCommand) Help() string {
	helpText := `
Usage: nomad queue status [options]

  View the current status of tenants in a batch job queue.

  When ACLs are enabled, this command requires a token with the 'list-jobs' capability. If multiple jobs in the queue are in different namespaces, the output will be filtered to only include jobs in namespaces the token has permissions for.
  
General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Eval Options:

  -limit
    The maximum number of tenants to return

  -verbose
    Display full output

  -json
    Display output as json

`
	return strings.TrimSpace(helpText)
}

func (c *QueueTenantsCommand) Synopsis() string {
	return "View the status of tenants in a batch job queue"
}

func (c *QueueTenantsCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-verbose": complete.PredictNothing,
			"-limit":   complete.PredictNothing,
			"-json":    complete.PredictNothing,
		})
}

func (c *QueueTenantsCommand) AutocompleteArgs() complete.Predictor {
	return JobPredictor(c.Meta.Client)
}

func (c *QueueTenantsCommand) Name() string { return "queue status" }

func (c *QueueTenantsCommand) Run(args []string) int {
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

	// Submit the request
	resp, _, err := client.BatchJobQueue().Tenants(qo)
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
		return c.printTenants(resp, jsonOut)
	case "unset":
		c.Ui.Output("No batch job queue configured")
	default:
		c.Ui.Error(fmt.Sprintf("Unknown queue type: %s", resp.Type))
		return 255
	}

	return 0
}

func (c *QueueTenantsCommand) printTenants(resp *api.BatchJobQueueTenantsResponse, jsonOut bool) int {
	tenants := []api.DynamicPriorityTenant{}
	bytes, err := json.Marshal(resp.Tenants)
	if err != nil {
		c.Ui.Error("Error marshaling response status")
		return 255
	}
	if err := json.Unmarshal(bytes, &tenants); err != nil {
		c.Ui.Error("Invalid Status response from server")
		return 255
	}

	slices.SortFunc(tenants, func(a, b api.DynamicPriorityTenant) int {
		return strings.Compare(a.TenantID, b.TenantID)
	})

	if !jsonOut {
		tenantInfo := []string{}
		tenantInfo = append(tenantInfo, "Tenant|Resource|Usage / Total|Percentage")

		for _, v := range tenants {
			tenantInfo = append(tenantInfo, fmt.Sprintf("%s|%s|%s|%d",
				v.TenantID,
				"",
				"",
				v.PercentageUsed,
			))

			resources := make([]string, 0, len(v.TenantUsage))
			for resource, usage := range v.TenantUsage {
				resources = append(resources, fmt.Sprintf("%s|%s|%.2f / %.2f|%s",
					"",
					resource,
					usage,
					v.TotalUsage[resource],
					"",
				))
			}
			slices.Sort(resources)
			tenantInfo = append(tenantInfo, resources...)
		}

		c.Ui.Output(c.Colorize().Color("[bold]Batch Queue Tenants[reset]"))
		c.Ui.Output(columnize.SimpleFormat(tenantInfo))
	} else {
		c.Ui.Output(string(bytes))
	}
	return 0
}
