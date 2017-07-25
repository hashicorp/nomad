package command

import (
	"fmt"
	"strings"
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

General Options:

  ` + generalOptionsUsage() + `

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

func (c *DeploymentFailCommand) Run(args []string) int {
	var detach, verbose bool

	flags := c.Meta.FlagSet("deployment fail", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
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
	if detach || !evalCreated {
		return 0
	}

	c.Ui.Output("")
	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(u.EvalID, false)
}
