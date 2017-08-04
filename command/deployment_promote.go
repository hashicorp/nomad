package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	flaghelper "github.com/hashicorp/nomad/helper/flag-helpers"
)

type DeploymentPromoteCommand struct {
	Meta
}

func (c *DeploymentPromoteCommand) Help() string {
	helpText := `
Usage: nomad deployment promote [options] <deployment id>

Promote is used to promote task groups in a deployment. Promotion should occur
when the deployment has placed canaries for a task group and those canaries have
been deemed healthy. When a task group is promoted, the rolling upgrade of the
remaining allocations is unblocked. If the canaries are found to be unhealthy,
the deployment may either be failed using the "nomad deployment fail" command,
the job can be failed forward by submitting a new version or failed backwards by
reverting to an older version using the "nomad job revert" command.

General Options:

  ` + generalOptionsUsage() + `

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

func (c *DeploymentPromoteCommand) Synopsis() string {
	return "Promote canaries in a deployment"
}

func (c *DeploymentPromoteCommand) Run(args []string) int {
	var detach, verbose bool
	var groups []string

	flags := c.Meta.FlagSet("deployment promote", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.Var((*flaghelper.StringFlag)(&groups), "group", "")

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

	var u *api.DeploymentUpdateResponse
	if len(groups) == 0 {
		u, _, err = client.Deployments().PromoteAll(deploy.ID, nil)
	} else {
		u, _, err = client.Deployments().PromoteGroups(deploy.ID, groups, nil)
	}

	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error promoting deployment: %s", err))
		return 1
	}

	// Nothing to do
	evalCreated := u.EvalID != ""
	if detach || !evalCreated {
		return 0
	}

	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(u.EvalID, false)
}
