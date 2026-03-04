// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	flaghelper "github.com/hashicorp/nomad/helper/flags"
	"github.com/mitchellh/go-glint"
	"github.com/mitchellh/go-glint/components"
	"github.com/posener/complete"
)

type JobDispatchCommand struct {
	Meta
}

func (c *JobDispatchCommand) Help() string {
	helpText := `
Usage: nomad job dispatch [options] <parameterized job> [input source]

  Dispatch creates an instance of a parameterized job. A data payload to the
  dispatched instance can be provided via stdin by using "-" or by specifying a
  path to a file. Metadata can be supplied by using the meta flag one or more
  times.

  An optional idempotency token can be used to prevent more than one instance
  of the job to be dispatched. If an instance with the same token already
  exists, the command returns without any action.

  Upon successful creation, the dispatched job ID will be printed and the
  triggered evaluation will be monitored. This can be disabled by supplying the
  detach flag.

  When ACLs are enabled, this command requires a token with the 'dispatch-job'
  capability for the job's namespace. The 'list-jobs' capability is required to
  run the command with a job prefix instead of the exact job ID. The 'read-job'
  capability is required to monitor the resulting evaluation when -detach is
  not used.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Dispatch Options:

  -meta <key>=<value>
    Meta takes a key/value pair separated by "=". The metadata key will be
    merged into the job's metadata. The job may define a default value for the
    key which is overridden when dispatching. The flag can be provided more than
    once to inject multiple metadata key/value pairs. Arbitrary keys are not
    allowed. The parameterized job must allow the key to be merged.

  -detach
    Return immediately instead of entering monitor mode. After job dispatch,
    the evaluation ID will be printed to the screen, which can be used to
    examine the evaluation using the eval-status command.

  -idempotency-token
    Optional identifier used to prevent more than one instance of the job from
    being dispatched.

  -id-prefix-template
    Optional prefix template for dispatched job IDs.

  -verbose
    Display full information.

  -wait
    Wait for all task groups to complete (all allocations reach terminal state).
    Without this flag, the command returns after the evaluation completes.

  -ui
    Open the dispatched job in the browser.
`
	return strings.TrimSpace(helpText)
}

func (c *JobDispatchCommand) Synopsis() string {
	return "Dispatch an instance of a parameterized job"
}

func (c *JobDispatchCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-meta":               complete.PredictAnything,
			"-detach":             complete.PredictNothing,
			"-idempotency-token":  complete.PredictAnything,
			"-verbose":            complete.PredictNothing,
			"-wait":               complete.PredictNothing,
			"-ui":                 complete.PredictNothing,
			"-id-prefix-template": complete.PredictAnything,
			"-priority":           complete.PredictAnything,
		})
}

func (c *JobDispatchCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Jobs().PrefixList(a.Last)
		if err != nil {
			return []string{}
		}

		// filter by parameterized jobs
		matches := make([]string, 0, len(resp))
		for _, job := range resp {
			if job.ParameterizedJob {
				matches = append(matches, job.ID)
			}
		}
		return matches

	})
}

func (c *JobDispatchCommand) Name() string { return "job dispatch" }

func (c *JobDispatchCommand) Run(args []string) int {
	var detach, verbose, wait, openURL bool
	var idempotencyToken string
	var meta []string
	var idPrefixTemplate string
	var priority int

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&wait, "wait", false, "")
	flags.StringVar(&idempotencyToken, "idempotency-token", "", "")
	flags.Var((*flaghelper.StringFlag)(&meta), "meta", "")
	flags.StringVar(&idPrefixTemplate, "id-prefix-template", "", "")
	flags.BoolVar(&openURL, "ui", false, "")
	flags.IntVar(&priority, "priority", 0, "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Check that we got one or two arguments
	args = flags.Args()
	if l := len(args); l < 1 || l > 2 {
		c.Ui.Error("This command takes one or two argument: <parameterized job> [input source]")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	var payload []byte
	var readErr error

	// Read the input
	if len(args) == 2 {
		switch args[1] {
		case "-":
			payload, readErr = io.ReadAll(os.Stdin)
		default:
			payload, readErr = os.ReadFile(args[1])
		}
		if readErr != nil {
			c.Ui.Error(fmt.Sprintf("Error reading input data: %v", readErr))
			return 1
		}
	}

	// Build the meta
	metaMap := make(map[string]string, len(meta))
	for _, m := range meta {
		split := strings.SplitN(m, "=", 2)
		if len(split) != 2 {
			c.Ui.Error(fmt.Sprintf("Error parsing meta value: %v", m))
			return 1
		}

		metaMap[split[0]] = split[1]
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check if the job exists
	jobIDPrefix := strings.TrimSpace(args[0])
	jobID, namespace, err := c.JobIDByPrefix(client, jobIDPrefix, func(j *api.JobListStub) bool {
		return j.ParameterizedJob
	})
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	// Dispatch the job
	w := &api.WriteOptions{
		IdempotencyToken: idempotencyToken,
		Namespace:        namespace,
	}
	opts := &api.DispatchOptions{
		JobID:            jobID,
		Meta:             metaMap,
		Payload:          payload,
		IdPrefixTemplate: idPrefixTemplate,
		Priority:         priority,
	}
	resp, _, err := client.Jobs().DispatchOpts(opts, w)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to dispatch job: %s", err))
		return 1
	}

	// See if an evaluation was created. If the job is periodic there will be no
	// eval.
	evalCreated := resp.EvalID != ""

	basic := []string{
		fmt.Sprintf("Dispatched Job ID|%s", resp.DispatchedJobID),
	}
	if evalCreated {
		basic = append(basic, fmt.Sprintf("Evaluation ID|%s", limit(resp.EvalID, length)))
	}
	c.Ui.Output(formatKV(basic))

	// Nothing to do
	if detach || !evalCreated {
		return 0
	}

	c.Ui.Output("")
	mon := newMonitor(c.Meta, client, length)

	// for hint purposes, need the dispatchedJobID to be escaped ("/" becomes "%2F")
	dispatchID := url.PathEscape(resp.DispatchedJobID)

	hint, _ := c.Meta.showUIPath(UIHintContext{
		Command: "job dispatch",
		PathParams: map[string]string{
			"dispatchID": dispatchID,
			"namespace":  namespace,
		},
		OpenURL: openURL,
	})
	if hint != "" {
		c.Ui.Warn(hint)
		// Because this is before monitor, newline so we don't scrunch
		c.Ui.Warn("")
	}

	// Monitor evaluation
	if mon.monitor(resp.EvalID) != 0 {
		return 1
	}

	// Monitor dispatched job allocations with deployment-style display (only if -wait flag is set)
	if wait {
		return c.monitorDispatchedJob(client, resp.DispatchedJobID, namespace, verbose, length)
	}

	return 0
}

// DispatchedJobState tracks the state of a dispatched job for a given task group.
type DispatchedJobState struct {
	ProgressDeadline  time.Duration
	RequireProgressBy time.Time
	DesiredTotal      int
	PlacedAllocs      int
	RunningAllocs     int
	FailedAllocs      int
}

func computeDispatchedJobStates(job *api.Job, allocs []*api.AllocationListStub,
) map[string]*DispatchedJobState {
	states := make(map[string]*DispatchedJobState)

	// Initialize states from job task groups
	for _, tg := range job.TaskGroups {
		tgName := *tg.Name
		states[tgName] = &DispatchedJobState{
			DesiredTotal:      *tg.Count,
			PlacedAllocs:      0,
			RunningAllocs:     0,
			FailedAllocs:      0,
			RequireProgressBy: time.Time{}, // Will be set to latest update
		}
	}

	// Compute metrics from allocations
	for _, alloc := range allocs {
		state, exists := states[alloc.TaskGroup]
		if !exists {
			continue
		}

		// Count placed allocations
		state.PlacedAllocs++

		// Count healthy (running) and unhealthy (failed) allocations
		switch alloc.ClientStatus {
		case api.AllocClientStatusRunning:
			state.RunningAllocs++
		case api.AllocClientStatusFailed:
			state.FailedAllocs++
		}

		// Track latest update time for progress deadline
		allocTime := time.Unix(0, alloc.ModifyTime)
		if allocTime.After(state.RequireProgressBy) {
			state.RequireProgressBy = allocTime
		}
	}

	return states
}

// monitorDispatchedJob monitors allocations with glint for interactive display
func (c *JobDispatchCommand) monitorDispatchedJob(
	client *api.Client,
	jobID string,
	namespace string,
	verbose bool,
	length int,
) int {
	/* 	// Query the dispatched job to get task group information
	   	q := &api.QueryOptions{
	   		Namespace: namespace,
	   	}
	   	job, _, err := client.Jobs().Info(jobID, q)
	   	if err != nil {
	   		c.Ui.Error(fmt.Sprintf("Error querying job: %s", err))
	   		return 1
	   	} */

	// Set up glint for interactive display
	d := glint.New()
	//defer d.Close()

	d.SetRefreshRate(100 * time.Millisecond)

	spinner := glint.Layout(
		components.Spinner(),
		glint.Text(fmt.Sprintf(" Monitoring allocations for job %q...", limit(jobID, length))),
	).Row()

	d.Set(spinner)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go d.Render(ctx)

	// Polling loop
	var lastIndex uint64
	startTime := time.Now()
	timeout := 5 * time.Minute
	d.Close()
	for {

		jobQuery := &api.QueryOptions{
			AllowStale: true,
			WaitIndex:  lastIndex,
			WaitTime:   5 * time.Second,
			Namespace:  namespace,
		}

		// ensure lastIndex is updated for next query even if error occurs
		job, meta, err := client.Jobs().Info(jobID, jobQuery)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying allocations: %s", err))
			return 1
		}
		lastIndex = meta.LastIndex

		allocQuery := &api.QueryOptions{
			AllowStale: true,
			Namespace:  namespace,
		}

		summary, _, err := client.Jobs().Summary(jobID, allocQuery)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying allocations: %s", err))
			return 1
		}

		// Build status component with Deployed section
		statusComponent := glint.Layout(
			glint.Text(""),
			glint.Style(glint.Text("Deployed"), glint.Bold()),
			glint.Text(formatTaskGroups(summary.Summary)),
		)

		// Add Allocations section if verbose
		if verbose {
			allocQuery := &api.QueryOptions{
				AllowStale: true,
				Namespace:  namespace,
			}
			allocs, _, err := client.Jobs().Allocations(jobID, true, allocQuery)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error querying allocations: %s", err))
				return 1
			}
			allocComponent := glint.Layout(
				glint.Text(""),
				glint.Style(glint.Text("Allocations"), glint.Bold()),
			)

			if len(allocs) > 0 {
				allocComponent = glint.Layout(
					allocComponent,
					glint.Text(formatAllocListStubs(allocs, true, length)),
				)
			} else {
				allocComponent = glint.Layout(
					allocComponent,
					glint.Text("No allocations created"),
				)
			}

			statusComponent = glint.Layout(statusComponent, allocComponent)
		}
		d := glint.New()
		// Add margin and update display
		statusComponent = glint.Layout(statusComponent).MarginLeft(4)
		d.Set(spinner, statusComponent)

		// Check if all task groups have completed
		d.Close()

		fmt.Println("Job Status:", *job.Status)
		if *job.Status == "dead" {
			for _, state := range summary.Summary {
				if state.Failed >= 0 {
					return 2
				}
			}
			return 0
		}

		// Check timeout
		if time.Since(startTime) > timeout {
			c.Ui.Warn(fmt.Sprintf("Timeout reached after %s", timeout))
			return 1
		}
	}
}

func formatTaskGroups(tgs map[string]api.TaskGroupSummary) string {

	tgNames := make([]string, 0, len(tgs))
	for name, _ := range tgs {
		tgNames = append(tgNames, name)
		/* 	if state.ProgressDeadline != 0 {
			progressDeadline = true
		} */
	}

	// Sort the task group names to get a reliable ordering
	sort.Strings(tgNames)

	// Build the row string
	rowString := "Task Group|"
	rowString += "Queued|"
	rowString += "Complete|Failed|Running|Starting|Lost|Unknown"

	rows := make([]string, len(tgs)+1)
	rows[0] = rowString
	i := 1

	for _, tg := range tgNames {
		state := tgs[tg]
		row := fmt.Sprintf("%s|", tg)
		row += fmt.Sprintf("%d|%d|%d|%d|%d|%d|%d", state.Queued, state.Complete, state.Failed,
			state.Running, state.Starting, state.Lost, state.Unknown)
		rows[i] = row
		i++
	}

	return formatList(rows)
}
