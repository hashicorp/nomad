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

type JobDeploymentsCommand struct {
	Meta
}

func (c *JobDeploymentsCommand) Help() string {
	helpText := `
Usage: nomad job deployments [options] <job>

  Deployments is used to display the deployments for a particular job.

  When ACLs are enabled, this command requires a token with the 'read-job'
  capability for the job's namespace. The 'list-jobs' capability is required to
  run the command with a job prefix instead of the exact job ID.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Deployments Options:

  -json
    Output the deployments in a JSON format.

  -t
    Format and display deployments using a Go template.

  -latest
    Display the latest deployment only.

  -verbose
    Display full information.

  -all
    Display all deployments matching the job ID, including those
    from an older instance of the job.
`
	return strings.TrimSpace(helpText)
}

func (c *JobDeploymentsCommand) Synopsis() string {
	return "List deployments for a job"
}

func (c *JobDeploymentsCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json":    complete.PredictNothing,
			"-t":       complete.PredictAnything,
			"-latest":  complete.PredictNothing,
			"-verbose": complete.PredictNothing,
			"-all":     complete.PredictNothing,
		})
}

func (c *JobDeploymentsCommand) AutocompleteArgs() complete.Predictor {
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

func (c *JobDeploymentsCommand) Name() string { return "job deployments" }

func (c *JobDeploymentsCommand) Run(args []string) int {
	var json, latest, verbose, all bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&latest, "latest", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&all, "all", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one node
	args = flags.Args()
	if l := len(args); l != 1 {
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

	// Check if the job exists
	jobIDPrefix := strings.TrimSpace(args[0])
	jobID, namespace, err := c.JobIDByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	q := &api.QueryOptions{Namespace: namespace}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	if latest {
		deploy, _, err := client.Jobs().LatestDeployment(jobID, q)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error retrieving deployments: %s", err))
			return 1
		}

		if json || len(tmpl) > 0 {
			out, err := Format(json, tmpl, deploy)
			if err != nil {
				c.Ui.Error(err.Error())
				return 1
			}

			c.Ui.Output(out)
			return 0
		}

		c.Ui.Output(c.Colorize().Color(formatDeployment(client, deploy, length)))
		return 0
	}

	deploys, _, err := client.Jobs().Deployments(jobID, all, q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving deployments: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, deploys)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatDeployments(deploys, length))
	return 0
}
