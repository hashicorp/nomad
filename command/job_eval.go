// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type JobEvalCommand struct {
	Meta
	forceRescheduling bool
}

func (c *JobEvalCommand) Help() string {
	helpText := `
Usage: nomad job eval [options] <job_id>

  Force an evaluation of the provided job ID. Forcing an evaluation will
  trigger the scheduler to re-evaluate the job. The force flags allow
  operators to force the scheduler to create new allocations under certain
  scenarios.

  When ACLs are enabled, this command requires a token with the 'submit-job'
  capability for the job's namespace. The 'list-jobs' capability is required to
  run the command with a job prefix instead of the exact job ID. The 'read-job'
  capability is required to monitor the resulting evaluation when -detach is
  not used.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Eval Options:

  -force-reschedule
    Force reschedule failed allocations even if they are not currently
    eligible for rescheduling.

  -detach
    Return immediately instead of entering monitor mode. The ID
    of the evaluation created will be printed to the screen, which can be
    used to examine the evaluation using the eval-status command.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobEvalCommand) Synopsis() string {
	return "Force an evaluation for the job"
}

func (c *JobEvalCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-force-reschedule": complete.PredictNothing,
			"-detach":           complete.PredictNothing,
			"-verbose":          complete.PredictNothing,
		})
}

func (c *JobEvalCommand) AutocompleteArgs() complete.Predictor {
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

func (c *JobEvalCommand) Name() string { return "job eval" }

func (c *JobEvalCommand) Run(args []string) int {
	var detach, verbose bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&c.forceRescheduling, "force-reschedule", false, "")
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no jobs or exactly one.
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <job>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

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

	// Check if the job exists
	jobIDPrefix := strings.TrimSpace(args[0])
	jobID, namespace, err := c.JobIDByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	// Call eval endpoint
	opts := api.EvalOptions{
		ForceReschedule: c.forceRescheduling,
	}
	w := &api.WriteOptions{
		Namespace: namespace,
	}
	evalId, _, err := client.Jobs().EvaluateWithOpts(jobID, opts, w)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error evaluating job: %s", err))
		return 1
	}

	if detach {
		c.Ui.Output(fmt.Sprintf("Created eval ID: %q ", limit(evalId, length)))
		return 0
	}

	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(evalId)
}
