// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gosuri/uilive"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/mitchellh/go-glint"
	"github.com/mitchellh/go-glint/components"
	"github.com/moby/term"
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

  -monitor
    Enter monitor mode to poll for updates to the deployment status.

  -wait
    How long to wait before polling an update, used in conjunction with monitor
    mode. Defaults to 2s.

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
			"-monitor": complete.PredictNothing,
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
	var json, verbose, monitor bool
	var wait time.Duration
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.BoolVar(&monitor, "monitor", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	flags.DurationVar(&wait, "wait", 2*time.Second, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that json or tmpl isn't set with monitor
	if monitor && (json || len(tmpl) > 0) {
		c.Ui.Error("The monitor flag cannot be used with the '-json' or '-t' flags")
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

	if monitor {
		// Call just to get meta
		_, meta, err := client.Deployments().Info(deploy.ID, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error retrieving deployment: %s", err))
		}

		c.Ui.Output(fmt.Sprintf("%s: Monitoring deployment %q",
			formatTime(time.Now()), limit(deploy.ID, length)))
		c.monitor(client, deploy.ID, meta.LastIndex, wait, verbose)

		return 0
	}
	c.Ui.Output(c.Colorize().Color(formatDeployment(client, deploy, length)))
	return 0
}

func (c *DeploymentStatusCommand) monitor(client *api.Client, deployID string, index uint64, wait time.Duration, verbose bool) (status string, err error) {
	if isStdoutTerminal() {
		return c.ttyMonitor(client, deployID, index, wait, verbose)
	} else {
		return c.defaultMonitor(client, deployID, index, wait, verbose)
	}
}

func isStdoutTerminal() bool {
	// TODO if/when glint offers full Windows support take out the runtime check
	if runtime.GOOS == "windows" {
		return false
	}

	// glint checks if the writer is a tty with additional
	// checks (e.g. terminal has non-0 size)
	r := &glint.TerminalRenderer{
		Output: os.Stdout,
	}

	return r.LayoutRoot() != nil
}

// Uses glint for printing in place. Same logic as the defaultMonitor function
// but only used for tty and non-Windows machines since glint doesn't work with
// cmd/PowerShell and non-interactive interfaces
// Margins are used to match the text alignment from job run
func (c *DeploymentStatusCommand) ttyMonitor(client *api.Client, deployID string, index uint64, wait time.Duration, verbose bool) (status string, err error) {
	var length int
	if verbose {
		length = fullId
	} else {
		length = shortId
	}

	d := glint.New()
	spinner := glint.Layout(
		components.Spinner(),
		glint.Text(fmt.Sprintf(" Deployment %q in progress...", limit(deployID, length))),
	).Row().MarginLeft(2)
	refreshRate := 100 * time.Millisecond

	d.SetRefreshRate(refreshRate)
	d.Set(spinner)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go d.Render(ctx)

	q := api.QueryOptions{
		AllowStale: true,
		WaitIndex:  index,
		WaitTime:   wait,
	}

	var statusComponent *glint.LayoutComponent
	var endSpinner *glint.LayoutComponent

UPDATE:
	for {
		var deploy *api.Deployment
		var meta *api.QueryMeta
		deploy, meta, err = client.Deployments().Info(deployID, &q)
		if err != nil {
			d.Append(glint.Layout(glint.Style(
				glint.Text(fmt.Sprintf("%s: Error fetching deployment: %v", formatTime(time.Now()), err)),
				glint.Color("red"),
			)).MarginLeft(4), glint.Text(""))
			d.RenderFrame()
			return
		}

		status = deploy.Status
		statusComponent = glint.Layout(
			glint.Text(""),
			glint.Text(formatTime(time.Now())),
			// Use colorize to render bold text in formatDeployment function
			glint.Text(c.Colorize().Color(formatDeployment(client, deploy, length))),
		)

		if verbose {
			allocComponent := glint.Layout(glint.Style(
				glint.Text("Allocations"),
				glint.Bold(),
			))

			allocs, _, err := client.Deployments().Allocations(deployID, nil)
			if err != nil {
				allocComponent = glint.Layout(
					allocComponent,
					glint.Style(
						glint.Text(fmt.Sprintf("Error fetching allocations: %v", err)),
						glint.Color("red"),
					),
				)
			} else {
				allocComponent = glint.Layout(
					allocComponent,
					glint.Text(formatAllocListStubs(allocs, verbose, length)),
				)
			}

			statusComponent = glint.Layout(
				statusComponent,
				glint.Text(""),
				allocComponent,
			)
		}

		statusComponent = glint.Layout(statusComponent).MarginLeft(4)
		d.Set(spinner, statusComponent)

		endSpinner = glint.Layout(
			components.Spinner(),
			glint.Text(fmt.Sprintf(" Deployment %q %s", limit(deployID, length), status)),
		).Row().MarginLeft(2)

		switch status {
		case api.DeploymentStatusFailed:
			if hasAutoRevert(deploy) {
				// Separate rollback monitoring from failed deployment
				d.Set(
					endSpinner,
					statusComponent,
					glint.Text(""),
				)

				// Wait for rollback to launch
				time.Sleep(1 * time.Second)
				var rollback *api.Deployment
				rollback, _, err = client.Jobs().LatestDeployment(deploy.JobID, nil)

				if err != nil {
					d.Append(glint.Layout(glint.Style(
						glint.Text(fmt.Sprintf("%s: Error fetching rollback deployment: %v", formatTime(time.Now()), err)),
						glint.Color("red"),
					)).MarginLeft(4), glint.Text(""))
					d.RenderFrame()
					return
				}

				// Check for noop/no target rollbacks
				// TODO We may want to find a more robust way of waiting for rollbacks to launch instead of
				// just sleeping for 1 sec. If scheduling is slow, this will break update here instead of
				// waiting for the (eventual) rollback
				if rollback == nil || rollback.ID == deploy.ID {
					break UPDATE
				}

				d.Close()
				c.ttyMonitor(client, rollback.ID, index, wait, verbose)
				return
			} else {
				endSpinner = glint.Layout(
					glint.Text(fmt.Sprintf("! Deployment %q %s", limit(deployID, length), status)),
				).Row().MarginLeft(2)
				break UPDATE
			}
		case api.DeploymentStatusSuccessful:
			endSpinner = glint.Layout(
				glint.Text(fmt.Sprintf("✓ Deployment %q %s", limit(deployID, length), status)),
			).Row().MarginLeft(2)
			break UPDATE
		case api.DeploymentStatusCancelled, api.DeploymentStatusBlocked:
			endSpinner = glint.Layout(
				glint.Text(fmt.Sprintf("! Deployment %q %s", limit(deployID, length), status)),
			).Row().MarginLeft(2)
			break UPDATE
		default:
			q.WaitIndex = meta.LastIndex
			continue
		}
	}
	// Render one final time with completion message
	d.Set(endSpinner, statusComponent, glint.Text(""))
	d.RenderFrame()
	return
}

// Used for Windows and non-tty
func (c *DeploymentStatusCommand) defaultMonitor(client *api.Client, deployID string, index uint64, wait time.Duration, verbose bool) (status string, err error) {
	writer := uilive.New()
	writer.Start()
	defer writer.Stop()

	var length int
	if verbose {
		length = fullId
	} else {
		length = shortId
	}

	q := api.QueryOptions{
		AllowStale: true,
		WaitIndex:  index,
		WaitTime:   wait,
	}

	for {
		var deploy *api.Deployment
		var meta *api.QueryMeta
		deploy, meta, err = client.Deployments().Info(deployID, &q)
		if err != nil {
			c.Ui.Error(c.Colorize().Color(fmt.Sprintf("%s: Error fetching deployment: %v", formatTime(time.Now()), err)))
			return
		}

		status = deploy.Status
		info := formatTime(time.Now())
		info += fmt.Sprintf("\n%s", formatDeployment(client, deploy, length))

		if verbose {
			info += "\n\n[bold]Allocations[reset]\n"
			allocs, _, err := client.Deployments().Allocations(deployID, nil)
			if err != nil {
				info += fmt.Sprintf("Error fetching allocations: %v", err)
			} else {
				info += formatAllocListStubs(allocs, verbose, length)
			}
		}

		// Add newline before output to avoid prefix indentation when called from job run
		msg := c.Colorize().Color(fmt.Sprintf("\n%s", info))

		// Print in place if tty
		_, isStdoutTerminal := term.GetFdInfo(os.Stdout)
		if isStdoutTerminal {
			fmt.Fprint(writer, msg)
		} else {
			c.Ui.Output(msg)
		}

		switch status {
		case api.DeploymentStatusFailed:
			if hasAutoRevert(deploy) {
				// Wait for rollback to launch
				time.Sleep(1 * time.Second)
				var rollback *api.Deployment
				rollback, _, err = client.Jobs().LatestDeployment(deploy.JobID, nil)

				// Separate rollback monitoring from failed deployment
				// Needs to be after time.Sleep or it messes up the formatting
				c.Ui.Output("")
				if err != nil {
					c.Ui.Error(c.Colorize().Color(
						fmt.Sprintf("%s: Error fetching deployment of previous job version: %v", formatTime(time.Now()), err),
					))
					return
				}

				// Check for noop/no target rollbacks
				// TODO We may want to find a more robust way of waiting for rollbacks to launch instead of
				// just sleeping for 1 sec. If scheduling is slow, this will break update here instead of
				// waiting for the (eventual) rollback
				if rollback == nil || rollback.ID == deploy.ID {
					return
				}
				c.defaultMonitor(client, rollback.ID, index, wait, verbose)
			}
			return

		case api.DeploymentStatusSuccessful, api.DeploymentStatusCancelled, api.DeploymentStatusBlocked:
			return
		default:
			q.WaitIndex = meta.LastIndex
			continue
		}
	}
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

	switch len(deploys) {
	case 0:
		return nil, nil, fmt.Errorf("Deployment ID %q matched no deployments", dID)
	case 1:
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
			base += "\n\nError fetching multiregion deployment\n\n"
			base += fmt.Sprintf("%v\n\n", err)
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
		return nil, fmt.Errorf("Error fetching job: %v", err)
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
		return nil, fmt.Errorf("Error fetching deployments for job %s: %v", d.JobID, err)
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

func hasAutoRevert(d *api.Deployment) bool {
	taskGroups := d.TaskGroups
	for _, state := range taskGroups {
		if state.AutoRevert {
			return true
		}
	}
	return false
}
