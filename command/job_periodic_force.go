// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type JobPeriodicForceCommand struct {
	Meta
}

func (c *JobPeriodicForceCommand) Help() string {
	helpText := `
Usage: nomad job periodic force <job id>

  This command is used to force the creation of a new instance of a periodic job.
  This is used to immediately run a periodic job, even if it violates the job's
  prohibit_overlap setting.

  When ACLs are enabled, this command requires a token with the 'submit-job'
  capability for the job's namespace. The 'list-jobs' capability is required to
  run the command with a job prefix instead of the exact job ID. The 'read-job'
  capability is required to monitor the resulting evaluation when -detach is
  not used.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Periodic Force Options:

  -detach
    Return immediately instead of entering monitor mode. After the force,
    the evaluation ID will be printed to the screen, which can be used to
    examine the evaluation using the eval-status command.

  -verbose
    Display full information.
`

	return strings.TrimSpace(helpText)
}

func (c *JobPeriodicForceCommand) Synopsis() string {
	return "Force the launch of a periodic job"
}

func (c *JobPeriodicForceCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-detach":  complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *JobPeriodicForceCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Jobs().PrefixList(a.Last)
		if err != nil {
			return []string{}
		}

		// filter this by periodic jobs
		matches := make([]string, 0, len(resp))
		for _, job := range resp {
			if job.Periodic {
				matches = append(matches, job.ID)
			}
		}
		return matches
	})
}

func (c *JobPeriodicForceCommand) Name() string { return "job periodic force" }

func (c *JobPeriodicForceCommand) Run(args []string) int {
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
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <job id>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check if the job exists
	jobIDPrefix := strings.TrimSpace(args[0])
	jobID, namespace, err := c.JobIDByPrefix(client, jobIDPrefix, func(j *api.JobListStub) bool {
		return j.Periodic
	})
	if err != nil {
		var noPrefixErr *NoJobWithPrefixError
		if errors.As(err, &noPrefixErr) {
			err = fmt.Errorf("No periodic job(s) with prefix or ID %q found", jobIDPrefix)
		}
		c.Ui.Error(err.Error())
		return 1
	}
	q := &api.WriteOptions{Namespace: namespace}

	// force the evaluation
	evalID, _, err := client.Jobs().PeriodicForce(jobID, q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error forcing periodic job %q: %s", jobID, err))
		return 1
	}

	if detach {
		c.Ui.Output("Force periodic successful")
		c.Ui.Output("Evaluation ID: " + evalID)
		return 0
	}

	// Detach was not specified, so start monitoring
	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(evalID)
}
