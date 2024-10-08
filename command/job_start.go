// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
	"os"
	"strings"
	"sync"
)

type JobStartCommand struct {
	Meta
}

func (c *JobStartCommand) Help() string {
	helpText := `
Usage: nomad job start [options] <job>
Alias: nomad start

  Start an existing stopped job. This command is used to start a previously stopped job's
  most recent running version up. Upon successful start, an interactive
  monitor session will start to display log lines as the job starts up its
  allocations based on its most recent running version. It is safe to exit the monitor
  early using ctrl+c.

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

	// Check that we got at least one job
	args = flags.Args()
	if len(args) < 1 {
		c.Ui.Error("This command takes at least one argument: <job>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	var jobIDs []string
	for _, jobID := range flags.Args() {
		jobIDs = append(jobIDs, strings.TrimSpace(jobID))
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	statusCh := make(chan int, len(jobIDs))
	var wg sync.WaitGroup

	for _, jobID := range jobIDs {
		jobID := jobID

		wg.Add(1)
		go func() {
			defer wg.Done()

			// Truncate the id unless full length is requested
			length := shortId
			if verbose {
				length = fullId
			}

			// Check if the job exists and has been stopped
			jobId, namespace, err := c.JobIDByPrefix(client, jobID, nil)
			if err != nil {
				c.Ui.Error(err.Error())
				statusCh <- 1
				return
			}
			job, err := c.JobByPrefix(client, jobId, nil)
			if err != nil {
				c.Ui.Error(err.Error())
				statusCh <- 1
				return
			}
			if *job.Status != "dead" {
				c.Ui.Error(fmt.Sprintf("Job  %v has not been stopped and has following status: %v", *job.Name, *job.Status))
				statusCh <- 1
				return

			}

			// Get all versions associated to current job
			q := &api.QueryOptions{Namespace: namespace}

			versions, _, _, err := client.Jobs().Versions(jobID, true, q)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error retrieving job versions: %s", err))
				statusCh <- 1
			}

			// Find the most recent running version for this job
			var chosenVersion *api.Job
			var chosenIndex uint64
			versionAvailable := false
			for i := len(versions) - 1; i >= 0; i-- {
				if *versions[i].Status == "running" {
					chosenVersion = versions[i]
					chosenIndex = uint64(i)
					versionAvailable = true
				}

			}
			if !versionAvailable {
				c.Ui.Error(fmt.Sprintf("No previous running versions of job %v,  %s", chosenVersion, err))
				statusCh <- 1
				return
			}

			// Parse the Consul token
			if consulToken == "" {
				// Check the environment variable
				consulToken = os.Getenv("CONSUL_HTTP_TOKEN")
			}

			// Parse the Vault token
			if vaultToken == "" {
				// Check the environment variable
				vaultToken = os.Getenv("VAULT_TOKEN")
			}

			// Revert to most recent running version!
			m := &api.WriteOptions{Namespace: namespace}
			resp, _, err := client.Jobs().Revert(jobID, chosenIndex, nil, m, consulToken, vaultToken)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error retrieving job versions: %s, %v", err, chosenIndex))
				statusCh <- 1
				return
			}
			if *job.Name == "bridge2" {
				c.Ui.Output(fmt.Sprintf("HERE"))

			}

			// Nothing to do
			evalCreated := resp.EvalID != ""

			if !evalCreated {
				statusCh <- 0
				return
			}

			if detach {
				c.Ui.Output("Evaluation ID: " + resp.EvalID)
				statusCh <- 0
				return
			}

			mon := newMonitor(c.Ui, client, length)
			statusCh <- mon.monitor(resp.EvalID)

		}()
	}
	// users will still see
	// errors if any while we
	// wait for the goroutines
	// to finish processing
	wg.Wait()

	// close the channel to ensure
	// the range statement below
	// doesn't go on indefinitely
	close(statusCh)

	// return a non-zero exit code
	// if even a single job start fails
	for status := range statusCh {
		if status != 0 {
			return status
		}
	}
	return 0
}
