// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	flaghelper "github.com/hashicorp/nomad/helper/flags"
	"github.com/posener/complete"
)

type JobPromoteCommand struct {
	Meta
}

func (c *JobPromoteCommand) Help() string {
	helpText := `
Usage: nomad job promote [options] <job id>

  Promote is used to promote task groups in the most recent deployment for the
  given job. Promotion should occur when the deployment has placed canaries for a
  task group and those canaries have been deemed healthy. When a task group is
  promoted, the rolling upgrade of the remaining allocations is unblocked. If the
  canaries are found to be unhealthy, the deployment may either be failed using
  the "nomad deployment fail" command, the job can be failed forward by submitting
  a new version or failed backwards by reverting to an older version using the
  "nomad job revert" command.

  When ACLs are enabled, this command requires a token with the 'submit-job',
  and 'read-job' capabilities for the job's namespace. The 'list-jobs'
  capability is required to run the command with a job prefix instead of the
  exact job ID.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Promote Options:

  -group
    Group may be specified many times and is used to promote that particular
    group. If no specific groups are specified, all groups are promoted.

  -detach
    Return immediately instead of entering monitor mode. After deployment
    resume, the evaluation ID will be printed to the screen, which can be used
    to examine the evaluation using the eval-status command.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobPromoteCommand) Synopsis() string {
	return "Promote a job's canaries"
}

func (c *JobPromoteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-group":   complete.PredictAnything,
			"-detach":  complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *JobPromoteCommand) AutocompleteArgs() complete.Predictor {
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

func (c *JobPromoteCommand) Name() string { return "job promote" }

func (c *JobPromoteCommand) Run(args []string) int {
	var detach, verbose bool
	var groups []string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.Var((*flaghelper.StringFlag)(&groups), "group", "")

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
	jobID, namespace, err := c.JobIDByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}
	q := &api.QueryOptions{Namespace: namespace}

	// Do a prefix lookup
	deploy, _, err := client.Jobs().LatestDeployment(jobID, q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving deployment: %s", err))
		return 1
	}

	if deploy == nil {
		c.Ui.Error(fmt.Sprintf("Job %q has no deployment to promote", jobID))
		return 1
	}

	wq := &api.WriteOptions{Namespace: namespace}
	var u *api.DeploymentUpdateResponse
	if len(groups) == 0 {
		u, _, err = client.Deployments().PromoteAll(deploy.ID, wq)
	} else {
		u, _, err = client.Deployments().PromoteGroups(deploy.ID, groups, wq)
	}

	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error promoting deployment %q for job %q: %s", deploy.ID, jobID, err))
		return 1
	}

	evalCreated := u.EvalID != ""

	// Nothing to do
	if !evalCreated {
		return 0
	}

	if detach {
		c.Ui.Output("Evaluation ID: " + u.EvalID)
		return 0
	}

	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(u.EvalID)
}
