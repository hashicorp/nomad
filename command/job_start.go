// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type JobStartCommand struct {
	Meta
}

func (c *JobStartCommand) Help() string {
	helpText := `
Usage: nomad job start [options] <job>
Alias: nomad start


 Starts a stopped job. The job must be currently registered with Nomad. Upon
 successful start, an interactive monitor session will start to display log
 lines as the job starts its allocations based on its most recent version.
 It is safe to exit the monitor early using ctrl+c.


 When ACLs are enabled, this command requires a token with the 'submit-job'
 and 'read-job' capabilities for the job's namespace. The 'list-jobs'
 capability is required to run the command with job prefixes instead of exact
 job IDs.


General Options:


 ` + generalOptionsUsage(usageOptsDefault) + `


Start Options:


 -detach
   Return immediately instead of entering monitor mode. After the
   job start command is submitted, a new evaluation ID is printed to the
   screen, which can be used to examine the evaluation using the eval-status
   command.


 -consul-token
  The Consul token used to verify that the caller has access to the Service
  Identity policies associated in the targeted version of the job.


 -vault-token
  The Vault token used to verify that the caller has access to the Vault
  policies in the targeted version of the job.


 -verbose
   Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobStartCommand) Synopsis() string {
	return "Start a stopped job"
}

func (c *JobStartCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-detach":  complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *JobStartCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Jobs, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Jobs]
	})
}
func (c *JobStartCommand) Name() string { return "job start" }

func (c *JobStartCommand) Run(args []string) int {
	var detach, verbose bool
	var consulToken, vaultToken string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.StringVar(&consulToken, "consul-token", "", "")
	flags.StringVar(&vaultToken, "vault-token", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one arguement
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <job>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	jobIDPrefix := strings.TrimSpace(args[0])

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}
	if consulToken == "" {
		consulToken = os.Getenv("CONSUL_HTTP_TOKEN")
	}

	if vaultToken == "" {
		vaultToken = os.Getenv("VAULT_TOKEN")
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	job, err := c.JobByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		fmt.Println("error getting job!")
		c.Ui.Error(err.Error())
		return 1
	}

	var jobID, jobName, namespace string

	if job.ID != nil {
		jobID = *job.ID
	}
	if job.Name != nil {
		jobName = *job.Name
	}
	if job.Namespace != nil {
		namespace = *job.Namespace
	}

	if job.Stop != nil && !*job.Stop {
		c.Ui.Error(fmt.Sprintf("Job '%v' has not been stopped", jobName))
		return 1
	}

	chosenVersion, err := c.GetSelectedVersion(client, jobID, namespace)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	// Revert to most recent non-stopped version!
	m := &api.WriteOptions{Namespace: namespace}
	resp, _, err := client.Jobs().Revert(jobID, chosenVersion, nil, m, consulToken, vaultToken)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving job version %v for job %s: %s,", chosenVersion, jobID, err))
		return 1
	}

	// Check if the job is periodic or is a parameterized job
	periodic := job.IsPeriodic()
	paramjob := job.IsParameterized()
	multiregion := job.IsMultiregion()
	if detach || periodic || paramjob || multiregion {
		c.Ui.Output("Job start successful")
		if periodic && !paramjob {
			loc, err := job.Periodic.GetLocation()
			if err == nil {
				now := time.Now().In(loc)
				next, err := job.Periodic.Next(now)
				if err != nil {
					c.Ui.Error(fmt.Sprintf("Error determining next launch time: %v", err))
				} else {
					c.Ui.Output(fmt.Sprintf("Approximate next launch time: %s (%s from now)",
						formatTime(next), formatTimeDifference(now, next, time.Second)))
				}
			}
		} else if !paramjob {
			c.Ui.Output("Evaluation ID: " + resp.EvalID)
		}

		return 0
	}

	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(resp.EvalID)
}

func (c *JobStartCommand) GetSelectedVersion(client *api.Client, jobID string, namespace string) (uint64, error) {

	// Get all versions associated to current job
	q := &api.QueryOptions{Namespace: namespace}

	// Versions are returned in sorted order
	versions, _, _, err := client.Jobs().Versions(jobID, true, q)
	if err != nil {
		return 0, fmt.Errorf("Error retrieving job versions: %s", err)
	}

	// Find the most recent version for this job that has not been stopped
	for _, version := range versions {
		if version.Stop == nil {
			continue
		}
		if !*version.Stop {
			return *version.Version, nil
		}
	}
	return 0, fmt.Errorf("No valid job version available to start")
}
