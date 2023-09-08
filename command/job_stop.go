// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type JobStopCommand struct {
	Meta
}

func (c *JobStopCommand) Help() string {
	helpText := `
Usage: nomad job stop [options] <job>
Alias: nomad stop

  Stop an existing job. This command is used to signal allocations to shut
  down for the given job ID. Upon successful deregistration, an interactive
  monitor session will start to display log lines as the job unwinds its
  allocations and completes shutting down. It is safe to exit the monitor
  early using ctrl+c.

  When ACLs are enabled, this command requires a token with the 'submit-job'
  and 'read-job' capabilities for the job's namespace. The 'list-jobs'
  capability is required to run the command with job prefixes instead of exact
  job IDs.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Stop Options:

  -detach
    Return immediately instead of entering monitor mode. After the
    deregister command is submitted, a new evaluation ID is printed to the
    screen, which can be used to examine the evaluation using the eval-status
    command.

  -eval-priority
    Override the priority of the evaluations produced as a result of this job
    deregistration. By default, this is set to the priority of the job.

  -global
    Stop a multi-region job in all its regions. By default job stop will stop
    only a single region at a time. Ignored for single-region jobs.

  -no-shutdown-delay
	Ignore the the group and task shutdown_delay configuration so that there is no
    delay between service deregistration and task shutdown. Note that using
    this flag will result in failed network connections to the allocations
    being stopped.

  -purge
    Purge is used to stop the job and purge it from the system. If not set, the
    job will still be queryable and will be purged by the garbage collector.

  -yes
    Automatic yes to prompts.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobStopCommand) Synopsis() string {
	return "Stop a running job"
}

func (c *JobStopCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-detach":            complete.PredictNothing,
			"-eval-priority":     complete.PredictNothing,
			"-purge":             complete.PredictNothing,
			"-global":            complete.PredictNothing,
			"-no-shutdown-delay": complete.PredictNothing,
			"-yes":               complete.PredictNothing,
			"-verbose":           complete.PredictNothing,
		})
}

func (c *JobStopCommand) AutocompleteArgs() complete.Predictor {
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

func (c *JobStopCommand) Name() string { return "job stop" }

func (c *JobStopCommand) Run(args []string) int {
	var detach, purge, verbose, global, autoYes, noShutdownDelay bool
	var evalPriority int

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&global, "global", false, "")
	flags.BoolVar(&noShutdownDelay, "no-shutdown-delay", false, "")
	flags.BoolVar(&autoYes, "yes", false, "")
	flags.BoolVar(&purge, "purge", false, "")
	flags.IntVar(&evalPriority, "eval-priority", 0, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one job
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

			// Check if the job exists
			job, err := c.JobByPrefix(client, jobID, nil)
			if err != nil {
				c.Ui.Error(err.Error())
				statusCh <- 1
				return
			}

			getConfirmation := func(question string) (int, bool) {
				answer, err := c.Ui.Ask(question)
				if err != nil {
					c.Ui.Error(fmt.Sprintf("Failed to parse answer: %v", err))
					return 1, false
				}

				if answer == "" || strings.ToLower(answer)[0] == 'n' {
					// No case
					c.Ui.Output("Cancelling job stop")
					return 0, false
				} else if strings.ToLower(answer)[0] == 'y' && len(answer) > 1 {
					// Non exact match yes
					c.Ui.Output("For confirmation, an exact ‘y’ is required.")
					return 0, false
				} else if answer != "y" {
					c.Ui.Output("No confirmation detected. For confirmation, an exact 'y' is required.")
					return 1, false
				}
				return 0, true
			}

			// Confirm the stop if the job was a prefix match
			// Ask for confirmation only when there's just one
			// job that needs to be stopped. Since we're stopping
			// jobs concurrently, we're going to skip confirmation
			// for when multiple jobs need to be stopped.
			if len(jobIDs) == 1 && jobID != *job.ID && !autoYes {
				question := fmt.Sprintf("Are you sure you want to stop job %q? [y/N]", *job.ID)
				code, confirmed := getConfirmation(question)
				if !confirmed {
					statusCh <- code
					return
				}
			}

			// Confirm we want to stop only a single region of a multiregion job
			if len(jobIDs) == 1 && job.IsMultiregion() && !global && !autoYes {
				question := fmt.Sprintf(
					"Are you sure you want to stop multi-region job %q in a single region? [y/N]", *job.ID)
				code, confirmed := getConfirmation(question)
				if !confirmed {
					statusCh <- code
					return
				}
			}

			// Invoke the stop
			opts := &api.DeregisterOptions{Purge: purge, Global: global, EvalPriority: evalPriority, NoShutdownDelay: noShutdownDelay}
			wq := &api.WriteOptions{Namespace: *job.Namespace}
			evalID, _, err := client.Jobs().DeregisterOpts(*job.ID, opts, wq)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error deregistering job with id %s err: %s", jobID, err))
				statusCh <- 1
				return
			}

			// If we are stopping a periodic job there won't be an evalID.
			if evalID == "" {
				statusCh <- 0
				return
			}

			// Goroutine won't wait on monitor
			if detach {
				c.Ui.Output(evalID)
				statusCh <- 0
				return
			}

			// Start monitoring the stop eval
			// and return result on status channel
			mon := newMonitor(c.Ui, client, length)
			statusCh <- mon.monitor(evalID)
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
	// if even a single job stop fails
	for status := range statusCh {
		if status != 0 {
			return status
		}
	}

	return 0
}
