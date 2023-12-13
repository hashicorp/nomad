// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type DeploymentFailCommand struct {
	Meta
}

func (c *DeploymentFailCommand) Help() string {
	helpText := `
Usage: nomad deployment fail [options] <deployment id>

  Fail is used to mark a deployment as failed. Failing a deployment will
  stop the placement of new allocations as part of rolling deployment and
  if the job is configured to auto revert, the job will attempt to roll back to a
  stable version.

  When ACLs are enabled, this command requires a token with the 'submit-job'
  and 'read-job' capabilities for the deployment's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Fail Options:

  -detach
    Return immediately instead of entering monitor mode. After deployment
    resume, the evaluation ID will be printed to the screen, which can be used
    to examine the evaluation using the eval-status command.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *DeploymentFailCommand) Synopsis() string {
	return "Manually fail a deployment"
}

func (c *DeploymentFailCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-detach":  complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *DeploymentFailCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Deployments, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Deployments]
	})
}

func (c *DeploymentFailCommand) Name() string { return "deployment fail" }

func (c *DeploymentFailCommand) Run(args []string) int {
	var detach, verbose bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <deployment id>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	dID := args[0]

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

	// Do a prefix lookup
	deploy, possible, err := getDeployment(client.Deployments(), dID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving deployment: %s", err))
		return 1
	}

	if len(possible) != 0 {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple deployments\n\n%s", formatDeployments(possible, length)))
		return 1
	}

	u, _, err := client.Deployments().Fail(deploy.ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error failing deployment: %s", err))
		return 1
	}

	if u.RevertedJobVersion == nil {
		c.Ui.Output(fmt.Sprintf("Deployment %q failed", deploy.ID))
	} else {
		c.Ui.Output(fmt.Sprintf("Deployment %q failed. Auto-reverted to job version %d.", deploy.ID, *u.RevertedJobVersion))
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

	c.Ui.Output("")
	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(u.EvalID)
}
