package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type DeploymentStatusCommand struct {
	Meta
}

func (c *DeploymentStatusCommand) Help() string {
	helpText := `
Usage: nomad deployment status [options] <deployment id>

Status is used to display the status of a deployment. The status will display
the number of desired changes as well as the currently applied changes.

General Options:

  ` + generalOptionsUsage() + `

Status Options:

  -verbose
    Display full information.

  -json
    Output the deployment in its JSON format.

  -t
    Format and display deployment using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *DeploymentStatusCommand) Synopsis() string {
	return "Display the status of a deployment"
}

func (c *DeploymentStatusCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-verbose": complete.PredictNothing,
			"-json":    complete.PredictNothing,
			"-t":       complete.PredictAnything,
		})
}

func (c *DeploymentStatusCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, _ := c.Meta.Client()
		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Deployments, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Deployments]
	})
}

func (c *DeploymentStatusCommand) Run(args []string) int {
	var json, verbose bool
	var tmpl string

	flags := c.Meta.FlagSet("deployment status", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

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

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, deploy)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(c.Colorize().Color(formatDeployment(deploy, length)))
	return 0
}

func getDeployment(client *api.Deployments, dID string) (match *api.Deployment, possible []*api.Deployment, err error) {
	// First attempt an immediate lookup if we have a proper length
	if len(dID) == 36 {
		d, _, err := client.Info(dID, nil)
		if err != nil {
			return nil, nil, err
		}

		return d, nil, nil
	}

	dID = strings.Replace(dID, "-", "", -1)
	if len(dID) == 1 {
		return nil, nil, fmt.Errorf("Identifier must contain at least two characters.")
	}
	if len(dID)%2 == 1 {
		// Identifiers must be of even length, so we strip off the last byte
		// to provide a consistent user experience.
		dID = dID[:len(dID)-1]
	}

	// Have to do a prefix lookup
	deploys, _, err := client.PrefixList(dID)
	if err != nil {
		return nil, nil, err
	}

	l := len(deploys)
	switch {
	case l == 0:
		return nil, nil, fmt.Errorf("Deployment ID %q matched no deployments", dID)
	case l == 1:
		return deploys[0], nil, nil
	default:
		return nil, deploys, nil
	}
}

func formatDeployment(d *api.Deployment, uuidLength int) string {
	// Format the high-level elements
	high := []string{
		fmt.Sprintf("ID|%s", limit(d.ID, uuidLength)),
		fmt.Sprintf("Job ID|%s", d.JobID),
		fmt.Sprintf("Job Version|%d", d.JobVersion),
		fmt.Sprintf("Status|%s", d.Status),
		fmt.Sprintf("Description|%s", d.StatusDescription),
	}

	base := formatKV(high)
	if len(d.TaskGroups) == 0 {
		return base
	}
	base += "\n\n[bold]Deployed[reset]\n"
	base += formatDeploymentGroups(d, uuidLength)
	return base
}

func formatDeploymentGroups(d *api.Deployment, uuidLength int) string {
	// Detect if we need to add these columns
	canaries, autorevert := false, false
	for _, state := range d.TaskGroups {
		if state.AutoRevert {
			autorevert = true
		}
		if state.DesiredCanaries > 0 {
			canaries = true
		}
	}

	// Build the row string
	rowString := "Task Group|"
	if autorevert {
		rowString += "Auto Revert|"
	}
	if canaries {
		rowString += "Promoted|"
	}
	rowString += "Desired|"
	if canaries {
		rowString += "Canaries|"
	}
	rowString += "Placed|Healthy|Unhealthy"

	rows := make([]string, len(d.TaskGroups)+1)
	rows[0] = rowString
	i := 1
	for tg, state := range d.TaskGroups {
		row := fmt.Sprintf("%s|", tg)
		if autorevert {
			row += fmt.Sprintf("%v|", state.AutoRevert)
		}
		if canaries {
			if state.DesiredCanaries > 0 {
				row += fmt.Sprintf("%v|", state.Promoted)
			} else {
				row += fmt.Sprintf("%v|", "N/A")
			}
		}
		row += fmt.Sprintf("%d|", state.DesiredTotal)
		if canaries {
			row += fmt.Sprintf("%d|", state.DesiredCanaries)
		}
		row += fmt.Sprintf("%d|%d|%d", state.PlacedAllocs, state.HealthyAllocs, state.UnhealthyAllocs)
		rows[i] = row
		i++
	}

	return formatList(rows)
}
