// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"time"

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

 Starts a stopped job. The job must be currently registered. Nomad will create a
 new version of the job based on its most recent version. Upon successful start,
 Nomad will enter an interactive monitor session. This is useful to watch
 Nomad's internals make scheduling decisions and place the submitted work onto
 nodes. The monitor will end once job placement is done. It is safe to exit the
 monitor early using ctrl+c.

 When ACLs are enabled, this command requires a token with the 'submit-job'
 capability for the job's namespace.

General Options:

 ` + generalOptionsUsage(usageOptsDefault) + `

Start Options:

 -detach
   Return immediately instead of entering monitor mode. After the
   job start command is submitted, a new evaluation ID is printed to the
   screen, which can be used to examine the evaluation using the eval-status
   command.

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

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one argument
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

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	job, err := c.JobByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	var jobName string

	if job.Name != nil {
		jobName = *job.Name
	}

	if job.Stop == nil || !*job.Stop {
		c.Ui.Error(fmt.Sprintf("Job %q has not been stopped", jobName))
		return 1
	}

	// register the job in a not stopped state
	*job.Stop = false

	resp, _, err := client.Jobs().Register(job, nil)

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
