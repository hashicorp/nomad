// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type DeploymentUnblockCommand struct {
	Meta
}

func (c *DeploymentUnblockCommand) Help() string {
	helpText := `
Usage: nomad deployment unblock [options] <deployment id>

  Unblock is used to unblock a multiregion deployment that's waiting for
  peer region deployments to complete.

  When ACLs are enabled, this command requires a token with the 'submit-job'
  and 'read-job' capabilities for the deployment's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Unblock Options:

  -detach
    Return immediately instead of entering monitor mode. After deployment
    unblock, the evaluation ID will be printed to the screen, which can be used
    to examine the evaluation using the eval-status command.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *DeploymentUnblockCommand) Synopsis() string {
	return "Unblock a blocked deployment"
}

func (c *DeploymentUnblockCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-detach":  complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *DeploymentUnblockCommand) AutocompleteArgs() complete.Predictor {
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

func (c *DeploymentUnblockCommand) Name() string { return "deployment unblock" }
func (c *DeploymentUnblockCommand) Run(args []string) int {
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

	u, _, err := client.Deployments().Unblock(deploy.ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error unblocking deployment: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Deployment %q unblocked", deploy.ID))
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
