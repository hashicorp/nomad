package command

import (
	"errors"
	"fmt"
	"sort"
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

  When ACLs are enabled, this command requires a token with the 'read-job'
  capability for the deployment's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

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

func (c *DeploymentStatusCommand) Name() string { return "deployment status" }

func (c *DeploymentStatusCommand) Run(args []string) int {
	var json, verbose bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one argument
	args = flags.Args()
	if l := len(args); l > 1 {
		c.Ui.Error("This command takes one argument: <deployment id>")
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

	// List if no arguments are provided
	if len(args) == 0 {
		deploys, _, err := client.Deployments().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error retrieving deployments: %s", err))
			return 1
		}

		c.Ui.Output(formatDeployments(deploys, length))
		return 0
	}

	// Do a prefix lookup
	dID := args[0]
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

	c.Ui.Output(c.Colorize().Color(formatDeployment(client, deploy, length)))
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

	dID = strings.ReplaceAll(dID, "-", "")
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

func formatDeployment(c *api.Client, d *api.Deployment, uuidLength int) string {
	if d == nil {
		return "No deployment found"
	}
	// Format the high-level elements
	high := []string{
		fmt.Sprintf("ID|%s", limit(d.ID, uuidLength)),
		fmt.Sprintf("Job ID|%s", d.JobID),
		fmt.Sprintf("Job Version|%d", d.JobVersion),
		fmt.Sprintf("Status|%s", d.Status),
		fmt.Sprintf("Description|%s", d.StatusDescription),
	}

	base := formatKV(high)

	// Fetch and Format Multi-region info
	if d.IsMultiregion {
		regions, err := fetchMultiRegionDeployments(c, d)
		if err != nil {
			base += "\n\nError fetching Multiregion deployments\n\n"
		} else if len(regions) > 0 {
			base += "\n\n[bold]Multiregion Deployment[reset]\n"
			base += formatMultiregionDeployment(regions, uuidLength)
		}
	}

	if len(d.TaskGroups) == 0 {
		return base
	}
	base += "\n\n[bold]Deployed[reset]\n"
	base += formatDeploymentGroups(d, uuidLength)
	return base
}

type regionResult struct {
	region string
	d      *api.Deployment
	err    error
}

func fetchMultiRegionDeployments(c *api.Client, d *api.Deployment) (map[string]*api.Deployment, error) {
	results := make(map[string]*api.Deployment)

	job, _, err := c.Jobs().Info(d.JobID, &api.QueryOptions{})
	if err != nil {
		return nil, err
	}

	requests := make(chan regionResult, len(job.Multiregion.Regions))
	for i := 0; i < cap(requests); i++ {
		go func(itr int) {
			region := job.Multiregion.Regions[itr]
			d, err := fetchRegionDeployment(c, d, region)
			requests <- regionResult{d: d, err: err, region: region.Name}
		}(i)
	}
	for i := 0; i < cap(requests); i++ {
		res := <-requests
		if res.err != nil {
			key := fmt.Sprintf("%s (error)", res.region)
			results[key] = &api.Deployment{}
			continue
		}
		results[res.region] = res.d

	}
	return results, nil
}

func fetchRegionDeployment(c *api.Client, d *api.Deployment, region *api.MultiregionRegion) (*api.Deployment, error) {
	if region == nil {
		return nil, errors.New("Region not found")
	}

	opts := &api.QueryOptions{Region: region.Name}
	deploys, _, err := c.Jobs().Deployments(d.JobID, false, opts)
	if err != nil {
		return nil, err
	}
	for _, dep := range deploys {
		if dep.JobVersion == d.JobVersion {
			return dep, nil
		}
	}
	return nil, fmt.Errorf("Could not find job version %d for region", d.JobVersion)
}

func formatMultiregionDeployment(regions map[string]*api.Deployment, uuidLength int) string {
	rowString := "Region|ID|Status"
	rows := make([]string, len(regions)+1)
	rows[0] = rowString
	i := 1
	for k, v := range regions {
		row := fmt.Sprintf("%s|%s|%s", k, limit(v.ID, uuidLength), v.Status)
		rows[i] = row
		i++
	}
	sort.Strings(rows)
	return formatList(rows)
}

func formatDeploymentGroups(d *api.Deployment, uuidLength int) string {
	// Detect if we need to add these columns
	var canaries, autorevert, progressDeadline bool
	tgNames := make([]string, 0, len(d.TaskGroups))
	for name, state := range d.TaskGroups {
		tgNames = append(tgNames, name)
		if state.AutoRevert {
			autorevert = true
		}
		if state.DesiredCanaries > 0 {
			canaries = true
		}
		if state.ProgressDeadline != 0 {
			progressDeadline = true
		}
	}

	// Sort the task group names to get a reliable ordering
	sort.Strings(tgNames)

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
	if progressDeadline {
		rowString += "|Progress Deadline"
	}

	rows := make([]string, len(d.TaskGroups)+1)
	rows[0] = rowString
	i := 1
	for _, tg := range tgNames {
		state := d.TaskGroups[tg]
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
		if progressDeadline {
			if state.RequireProgressBy.IsZero() {
				row += fmt.Sprintf("|%v", "N/A")
			} else {
				row += fmt.Sprintf("|%v", formatTime(state.RequireProgressBy))
			}
		}
		rows[i] = row
		i++
	}

	return formatList(rows)
}
