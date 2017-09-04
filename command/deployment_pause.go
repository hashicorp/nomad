package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type DeploymentPauseCommand struct {
	Meta
}

func (c *DeploymentPauseCommand) Help() string {
	helpText := `
Usage: nomad deployment pause [options] <deployment id>

Pause is used to pause a deployment. Pausing a deployment will pause the
placement of new allocations as part of rolling deployment.

General Options:

  ` + generalOptionsUsage() + `

Pause Options:

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *DeploymentPauseCommand) Synopsis() string {
	return "Pause a deployment"
}

func (c *DeploymentPauseCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-verbose": complete.PredictNothing,
		})
}

func (c *DeploymentPauseCommand) AutocompleteArgs() complete.Predictor {
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

func (c *DeploymentPauseCommand) Run(args []string) int {
	var verbose bool

	flags := c.Meta.FlagSet("deployment pause", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error(c.Help())
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

	if _, _, err := client.Deployments().Pause(deploy.ID, true, nil); err != nil {
		c.Ui.Error(fmt.Sprintf("Error pausing deployment: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Deployment %q paused", deploy.ID))
	return 0
}
