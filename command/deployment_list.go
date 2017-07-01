package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
)

type DeploymentListCommand struct {
	Meta
}

func (c *DeploymentListCommand) Help() string {
	helpText := `
Usage: nomad deployment list [options]

List is used to list the set of deployments tracked by Nomad.

General Options:

  ` + generalOptionsUsage() + `

List Options:

  -json
    Output the deployments in a JSON format.

  -t
    Format and display the deployments using a Go template.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *DeploymentListCommand) Synopsis() string {
	return "List all deployments"
}

func (c *DeploymentListCommand) Run(args []string) int {
	var json, verbose bool
	var tmpl string

	flags := c.Meta.FlagSet("deployment list", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error(c.Help())
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

	deploys, _, err := client.Deployments().List(nil)
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

func formatDeployments(deploys []*api.Deployment, uuidLength int) string {
	if len(deploys) == 0 {
		return "No deployments found"
	}

	rows := make([]string, len(deploys)+1)
	rows[0] = "ID|Job ID|Job Version|Status|Description"
	for i, d := range deploys {
		rows[i+1] = fmt.Sprintf("%s|%s|%d|%s|%s",
			limit(d.ID, uuidLength),
			d.JobID,
			d.JobVersion,
			d.Status,
			d.StatusDescription)
	}
	return formatList(rows)
}
