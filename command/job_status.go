// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

const (
	// maxFailedTGs is the maximum number of task groups we show failure reasons
	// for before deferring to eval-status
	maxFailedTGs = 5
)

type JobStatusCommand struct {
	Meta
	length    int
	evals     bool
	allAllocs bool
	verbose   bool
	json      bool
	tmpl      string
}

// NamespacedID is a tuple of an ID and a namespace
type NamespacedID struct {
	ID        string
	Namespace string
}

type JobJson struct {
	Summary          *api.JobSummary
	Allocations      []*api.AllocationListStub
	LatestDeployment *api.Deployment
	Evaluations      []*api.Evaluation
}

func (c *JobStatusCommand) Help() string {
	helpText := `
Usage: nomad job status [options] <job>

  Display status information about a job. If no job ID is given, a list of all
  known jobs will be displayed.

  When ACLs are enabled, this command requires a token with the 'read-job'
  capability for the job's namespace. The 'list-jobs' capability is required to
  run the command with a job prefix instead of the exact job ID.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Status Options:

  -short
    Display short output. Used only when a single job is being
    queried, and drops verbose information about allocations.

  -evals
    Display the evaluations associated with the job.

  -all-allocs
    Display all allocations matching the job ID, including those from an older
    instance of the job.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobStatusCommand) Synopsis() string {
	return "Display status information about a job"
}

func (c *JobStatusCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-all-allocs": complete.PredictNothing,
			"-evals":      complete.PredictNothing,
			"-short":      complete.PredictNothing,
			"-verbose":    complete.PredictNothing,
		})
}

func (c *JobStatusCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Jobs, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Jobs]
	})
}

func (c *JobStatusCommand) Name() string { return "status" }

func (c *JobStatusCommand) Run(args []string) int {
	var short bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&short, "short", false, "")
	flags.BoolVar(&c.evals, "evals", false, "")
	flags.BoolVar(&c.allAllocs, "all-allocs", false, "")
	flags.BoolVar(&c.json, "json", false, "")
	flags.StringVar(&c.tmpl, "t", "", "")
	flags.BoolVar(&c.verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no jobs or exactly one.
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error("This command takes either no arguments or one: <job>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Truncate the id unless full length is requested
	c.length = shortId
	if c.verbose {
		c.length = fullId
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	allNamespaces := c.allNamespaces()

	// Invoke list mode if no job ID.
	if len(args) == 0 {
		jobs, _, err := client.Jobs().ListOptions(nil, nil)

		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying jobs: %s", err))
			return 1
		}

		if len(jobs) == 0 {
			// No output if we have no jobs
			c.Ui.Output("No running jobs")
		} else {
			if c.json || len(c.tmpl) > 0 {
				pairs := make([]NamespacedID, len(jobs))

				for i, j := range jobs {
					pairs[i] = NamespacedID{ID: j.ID, Namespace: j.Namespace}
				}

				jsonJobs, err := createJsonJobsOutput(client, c.allAllocs, pairs...)
				if err != nil {
					c.Ui.Error(err.Error())
					return 1
				}

				out, err := Format(c.json, c.tmpl, jsonJobs)
				if err != nil {
					c.Ui.Error(err.Error())
					return 1
				}

				c.Ui.Output(out)
			} else {
				c.Ui.Output(createStatusListOutput(jobs, allNamespaces))
			}
		}
		return 0
	}

	// Try querying the job
	jobIDPrefix := strings.TrimSpace(args[0])
	jobID, namespace, err := c.JobIDByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	// Prefix lookup matched a single job
	q := &api.QueryOptions{Namespace: namespace}
	job, _, err := client.Jobs().Info(jobID, q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying job: %s", err))
		return 1
	}

	periodic := job.IsPeriodic()
	parameterized := job.IsParameterized()

	nodePool := ""
	if job.NodePool != nil {
		nodePool = *job.NodePool
	}

	if c.json || len(c.tmpl) > 0 {
		jsonJobs, err := createJsonJobsOutput(client, c.allAllocs,
			NamespacedID{ID: *job.ID, Namespace: *job.Namespace})

		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		out, err := Format(c.json, c.tmpl, jsonJobs)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)

		return 0
	}

	// Format the job info
	basic := []string{
		fmt.Sprintf("ID|%s", *job.ID),
		fmt.Sprintf("Name|%s", *job.Name),
		fmt.Sprintf("Submit Date|%s", formatTime(time.Unix(0, *job.SubmitTime))),
		fmt.Sprintf("Type|%s", *job.Type),
		fmt.Sprintf("Priority|%d", *job.Priority),
		fmt.Sprintf("Datacenters|%s", strings.Join(job.Datacenters, ",")),
		fmt.Sprintf("Namespace|%s", *job.Namespace),
		fmt.Sprintf("Node Pool|%s", nodePool),
		fmt.Sprintf("Status|%s", getStatusString(*job.Status, job.Stop)),
		fmt.Sprintf("Periodic|%v", periodic),
		fmt.Sprintf("Parameterized|%v", parameterized),
	}

	if job.DispatchIdempotencyToken != nil && *job.DispatchIdempotencyToken != "" {
		basic = append(basic, fmt.Sprintf("Idempotency Token|%v", *job.DispatchIdempotencyToken))
	}

	if periodic && !parameterized {
		if *job.Stop {
			basic = append(basic, "Next Periodic Launch|none (job stopped)")
		} else {
			location, err := job.Periodic.GetLocation()
			if err == nil {
				now := time.Now().In(location)
				next, err := job.Periodic.Next(now)
				if err == nil {
					basic = append(basic, fmt.Sprintf("Next Periodic Launch|%s",
						fmt.Sprintf("%s (%s from now)",
							formatTime(next), formatTimeDifference(now, next, time.Second))))
				}
			}
		}
	}

	c.Ui.Output(formatKV(basic))

	// Exit early
	if short {
		return 0
	}

	// Print periodic job information
	if periodic && !parameterized {
		if err := c.outputPeriodicInfo(client, job); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	} else if parameterized {
		if err := c.outputParameterizedInfo(client, job); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	} else {
		if err := c.outputJobInfo(client, job); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	}

	return 0
}

// outputPeriodicInfo prints information about the passed periodic job. If a
// request fails, an error is returned.
func (c *JobStatusCommand) outputPeriodicInfo(client *api.Client, job *api.Job) error {
	// Output the summary
	if err := c.outputJobSummary(client, job); err != nil {
		return err
	}

	// Generate the prefix that matches launched jobs from the periodic job.
	prefix := fmt.Sprintf("%s%s", *job.ID, api.JobPeriodicLaunchSuffix)
	children, _, err := client.Jobs().PrefixList(prefix)
	if err != nil {
		return fmt.Errorf("Error querying job: %s", err)
	}

	if len(children) == 0 {
		c.Ui.Output("\nNo instances of periodic job found")
		return nil
	}

	out := make([]string, 1)
	out[0] = "ID|Status"
	for _, child := range children {
		// Ensure that we are only showing jobs whose parent is the requested
		// job.
		if child.ParentID != *job.ID {
			continue
		}

		out = append(out, fmt.Sprintf("%s|%s",
			child.ID,
			child.Status))
	}

	c.Ui.Output(c.Colorize().Color("\n[bold]Previously Launched Jobs[reset]"))
	c.Ui.Output(formatList(out))
	return nil
}

// outputParameterizedInfo prints information about a parameterized job. If a
// request fails, an error is returned.
func (c *JobStatusCommand) outputParameterizedInfo(client *api.Client, job *api.Job) error {
	// Output parameterized job details
	c.Ui.Output(c.Colorize().Color("\n[bold]Parameterized Job[reset]"))
	parameterizedJob := make([]string, 3)
	parameterizedJob[0] = fmt.Sprintf("Payload|%s", job.ParameterizedJob.Payload)
	parameterizedJob[1] = fmt.Sprintf("Required Metadata|%v", strings.Join(job.ParameterizedJob.MetaRequired, ", "))
	parameterizedJob[2] = fmt.Sprintf("Optional Metadata|%v", strings.Join(job.ParameterizedJob.MetaOptional, ", "))
	c.Ui.Output(formatKV(parameterizedJob))

	// Output the summary
	if err := c.outputJobSummary(client, job); err != nil {
		return err
	}

	// Generate the prefix that matches launched jobs from the parameterized job.
	prefix := fmt.Sprintf("%s%s", *job.ID, api.JobDispatchLaunchSuffix)
	children, _, err := client.Jobs().PrefixList(prefix)
	if err != nil {
		return fmt.Errorf("Error querying job: %s", err)
	}

	if len(children) == 0 {
		c.Ui.Output("\nNo dispatched instances of parameterized job found")
		return nil
	}

	out := make([]string, 1)
	out[0] = "ID|Status"
	for _, child := range children {
		// Ensure that we are only showing jobs whose parent is the requested
		// job.
		if child.ParentID != *job.ID {
			continue
		}

		out = append(out, fmt.Sprintf("%s|%s",
			child.ID,
			child.Status))
	}

	c.Ui.Output(c.Colorize().Color("\n[bold]Dispatched Jobs[reset]"))
	c.Ui.Output(formatList(out))
	return nil
}

// outputJobInfo prints information about the passed non-periodic job. If a
// request fails, an error is returned.
func (c *JobStatusCommand) outputJobInfo(client *api.Client, job *api.Job) error {
	var q *api.QueryOptions
	if job.Namespace != nil {
		q = &api.QueryOptions{Namespace: *job.Namespace}
	}

	// Query the allocations
	jobAllocs, _, err := client.Jobs().Allocations(*job.ID, c.allAllocs, q)
	if err != nil {
		return fmt.Errorf("Error querying job allocations: %s", err)
	}

	// Query the evaluations
	jobEvals, _, err := client.Jobs().Evaluations(*job.ID, q)
	if err != nil {
		return fmt.Errorf("Error querying job evaluations: %s", err)
	}

	latestDeployment, _, err := client.Jobs().LatestDeployment(*job.ID, q)
	if err != nil {
		return fmt.Errorf("Error querying latest job deployment: %s", err)
	}

	// Output the summary
	if err := c.outputJobSummary(client, job); err != nil {
		return err
	}

	// Determine latest evaluation with failures whose follow up hasn't
	// completed, this is done while formatting
	var latestFailedPlacement *api.Evaluation
	blockedEval := false

	// Format the evals
	evals := make([]string, len(jobEvals)+1)
	evals[0] = "ID|Priority|Triggered By|Status|Placement Failures"
	for i, eval := range jobEvals {
		failures, _ := evalFailureStatus(eval)
		evals[i+1] = fmt.Sprintf("%s|%d|%s|%s|%s",
			limit(eval.ID, c.length),
			eval.Priority,
			eval.TriggeredBy,
			eval.Status,
			failures,
		)

		if eval.Status == "blocked" {
			blockedEval = true
		}

		if len(eval.FailedTGAllocs) == 0 {
			// Skip evals without failures
			continue
		}

		if latestFailedPlacement == nil || latestFailedPlacement.CreateIndex < eval.CreateIndex {
			latestFailedPlacement = eval
		}
	}

	if c.verbose || c.evals {
		c.Ui.Output(c.Colorize().Color("\n[bold]Evaluations[reset]"))
		c.Ui.Output(formatList(evals))
	}

	if blockedEval && latestFailedPlacement != nil {
		c.outputFailedPlacements(latestFailedPlacement)
	}

	c.outputReschedulingEvals(client, job, jobAllocs, c.length)

	if latestDeployment != nil {
		c.Ui.Output(c.Colorize().Color("\n[bold]Latest Deployment[reset]"))
		c.Ui.Output(c.Colorize().Color(c.formatDeployment(client, latestDeployment)))
	}

	// Format the allocs
	c.Ui.Output(c.Colorize().Color("\n[bold]Allocations[reset]"))
	c.Ui.Output(formatAllocListStubs(jobAllocs, c.verbose, c.length))
	return nil
}

func (c *JobStatusCommand) formatDeployment(client *api.Client, d *api.Deployment) string {
	// Format the high-level elements
	high := []string{
		fmt.Sprintf("ID|%s", limit(d.ID, c.length)),
		fmt.Sprintf("Status|%s", d.Status),
		fmt.Sprintf("Description|%s", d.StatusDescription),
	}

	base := formatKV(high)

	if d.IsMultiregion {
		regions, err := fetchMultiRegionDeployments(client, d)
		if err != nil {
			base += "\n\nError fetching Multiregion deployments\n\n"
		} else if len(regions) > 0 {
			base += "\n\n[bold]Multiregion Deployment[reset]\n"
			base += formatMultiregionDeployment(regions, 8)
		}
	}

	if len(d.TaskGroups) == 0 {
		return base
	}
	base += "\n\n[bold]Deployed[reset]\n"
	base += formatDeploymentGroups(d, c.length)
	return base
}

func formatAllocListStubs(stubs []*api.AllocationListStub, verbose bool, uuidLength int) string {
	if len(stubs) == 0 {
		return "No allocations placed"
	}

	allocs := make([]string, len(stubs)+1)
	if verbose {
		allocs[0] = "ID|Eval ID|Node ID|Node Name|Task Group|Version|Desired|Status|Created|Modified"
		for i, alloc := range stubs {
			allocs[i+1] = fmt.Sprintf("%s|%s|%s|%s|%s|%d|%s|%s|%s|%s",
				limit(alloc.ID, uuidLength),
				limit(alloc.EvalID, uuidLength),
				limit(alloc.NodeID, uuidLength),
				alloc.NodeName,
				alloc.TaskGroup,
				alloc.JobVersion,
				alloc.DesiredStatus,
				alloc.ClientStatus,
				formatUnixNanoTime(alloc.CreateTime),
				formatUnixNanoTime(alloc.ModifyTime))
		}
	} else {
		allocs[0] = "ID|Node ID|Task Group|Version|Desired|Status|Created|Modified"
		for i, alloc := range stubs {
			now := time.Now()
			createTimePretty := prettyTimeDiff(time.Unix(0, alloc.CreateTime), now)
			modTimePretty := prettyTimeDiff(time.Unix(0, alloc.ModifyTime), now)
			allocs[i+1] = fmt.Sprintf("%s|%s|%s|%d|%s|%s|%s|%s",
				limit(alloc.ID, uuidLength),
				limit(alloc.NodeID, uuidLength),
				alloc.TaskGroup,
				alloc.JobVersion,
				alloc.DesiredStatus,
				alloc.ClientStatus,
				createTimePretty,
				modTimePretty)
		}
	}

	return formatList(allocs)
}

func formatAllocList(allocations []*api.Allocation, verbose bool, uuidLength int) string {
	if len(allocations) == 0 {
		return "No allocations placed"
	}

	allocs := make([]string, len(allocations)+1)
	if verbose {
		allocs[0] = "ID|Eval ID|Node ID|Task Group|Version|Desired|Status|Created|Modified"
		for i, alloc := range allocations {
			allocs[i+1] = fmt.Sprintf("%s|%s|%s|%s|%d|%s|%s|%s|%s",
				limit(alloc.ID, uuidLength),
				limit(alloc.EvalID, uuidLength),
				limit(alloc.NodeID, uuidLength),
				alloc.TaskGroup,
				*alloc.Job.Version,
				alloc.DesiredStatus,
				alloc.ClientStatus,
				formatUnixNanoTime(alloc.CreateTime),
				formatUnixNanoTime(alloc.ModifyTime))
		}
	} else {
		allocs[0] = "ID|Node ID|Task Group|Version|Desired|Status|Created|Modified"
		for i, alloc := range allocations {
			now := time.Now()
			createTimePretty := prettyTimeDiff(time.Unix(0, alloc.CreateTime), now)
			modTimePretty := prettyTimeDiff(time.Unix(0, alloc.ModifyTime), now)
			allocs[i+1] = fmt.Sprintf("%s|%s|%s|%d|%s|%s|%s|%s",
				limit(alloc.ID, uuidLength),
				limit(alloc.NodeID, uuidLength),
				alloc.TaskGroup,
				*alloc.Job.Version,
				alloc.DesiredStatus,
				alloc.ClientStatus,
				createTimePretty,
				modTimePretty)
		}
	}

	return formatList(allocs)
}

// outputJobSummary displays the given jobs summary and children job summary
// where appropriate
func (c *JobStatusCommand) outputJobSummary(client *api.Client, job *api.Job) error {
	// Query the summary
	q := &api.QueryOptions{Namespace: *job.Namespace}
	summary, _, err := client.Jobs().Summary(*job.ID, q)
	if err != nil {
		return fmt.Errorf("Error querying job summary: %s", err)
	}

	if summary == nil {
		return nil
	}

	periodic := job.IsPeriodic()
	parameterizedJob := job.IsParameterized()

	// Print the summary
	if !periodic && !parameterizedJob {
		c.Ui.Output(c.Colorize().Color("\n[bold]Summary[reset]"))
		summaries := make([]string, len(summary.Summary)+1)
		summaries[0] = "Task Group|Queued|Starting|Running|Failed|Complete|Lost|Unknown"
		taskGroups := make([]string, 0, len(summary.Summary))
		for taskGroup := range summary.Summary {
			taskGroups = append(taskGroups, taskGroup)
		}
		sort.Strings(taskGroups)
		for idx, taskGroup := range taskGroups {
			tgs := summary.Summary[taskGroup]
			summaries[idx+1] = fmt.Sprintf("%s|%d|%d|%d|%d|%d|%d|%d",
				taskGroup, tgs.Queued, tgs.Starting,
				tgs.Running, tgs.Failed,
				tgs.Complete, tgs.Lost, tgs.Unknown,
			)
		}
		c.Ui.Output(formatList(summaries))
	}

	// Always display the summary if we are periodic or parameterized, but
	// only display if the summary is non-zero on normal jobs
	if summary.Children != nil && (parameterizedJob || periodic || summary.Children.Sum() > 0) {
		if parameterizedJob {
			c.Ui.Output(c.Colorize().Color("\n[bold]Parameterized Job Summary[reset]"))
		} else {
			c.Ui.Output(c.Colorize().Color("\n[bold]Children Job Summary[reset]"))
		}
		summaries := make([]string, 2)
		summaries[0] = "Pending|Running|Dead"
		summaries[1] = fmt.Sprintf("%d|%d|%d",
			summary.Children.Pending, summary.Children.Running, summary.Children.Dead)
		c.Ui.Output(formatList(summaries))
	}

	return nil
}

// outputReschedulingEvals displays eval IDs and time for any
// delayed evaluations by task group
func (c *JobStatusCommand) outputReschedulingEvals(client *api.Client, job *api.Job, allocListStubs []*api.AllocationListStub, uuidLength int) error {
	// Get the most recent alloc ID by task group

	mostRecentAllocs := make(map[string]*api.AllocationListStub)
	for _, alloc := range allocListStubs {
		a, ok := mostRecentAllocs[alloc.TaskGroup]
		if !ok || alloc.ModifyTime > a.ModifyTime {
			mostRecentAllocs[alloc.TaskGroup] = alloc
		}
	}

	followUpEvalIds := make(map[string]string)
	for tg, alloc := range mostRecentAllocs {
		if alloc.FollowupEvalID != "" {
			followUpEvalIds[tg] = alloc.FollowupEvalID
		}
	}

	if len(followUpEvalIds) == 0 {
		return nil
	}
	// Print the reschedule info section
	var delayedEvalInfos []string

	taskGroups := make([]string, 0, len(followUpEvalIds))
	for taskGroup := range followUpEvalIds {
		taskGroups = append(taskGroups, taskGroup)
	}
	sort.Strings(taskGroups)
	var evalDetails []string
	first := true
	for _, taskGroup := range taskGroups {
		evalID := followUpEvalIds[taskGroup]
		evaluation, _, err := client.Evaluations().Info(evalID, nil)
		// Eval time is not critical output,
		// so don't return it on errors, if its not set, or its already in the past
		if err != nil || evaluation.WaitUntil.IsZero() || time.Now().After(evaluation.WaitUntil) {
			continue
		}
		evalTime := prettyTimeDiff(evaluation.WaitUntil, time.Now())
		if c.verbose {
			if first {
				delayedEvalInfos = append(delayedEvalInfos, "Task Group|Reschedule Policy|Eval ID|Eval Time")
			}
			rp := job.LookupTaskGroup(taskGroup).ReschedulePolicy
			evalDetails = append(evalDetails, fmt.Sprintf("%s|%s|%s|%s", taskGroup, rp.String(), limit(evalID, uuidLength), evalTime))
		} else {
			if first {
				delayedEvalInfos = append(delayedEvalInfos, "Task Group|Eval ID|Eval Time")
			}
			evalDetails = append(evalDetails, fmt.Sprintf("%s|%s|%s", taskGroup, limit(evalID, uuidLength), evalTime))
		}
		first = false
	}
	if len(evalDetails) == 0 {
		return nil
	}
	// Only show this section if there is pending evals
	delayedEvalInfos = append(delayedEvalInfos, evalDetails...)
	c.Ui.Output(c.Colorize().Color("\n[bold]Future Rescheduling Attempts[reset]"))
	c.Ui.Output(formatList(delayedEvalInfos))
	return nil
}

func (c *JobStatusCommand) outputFailedPlacements(failedEval *api.Evaluation) {
	if failedEval == nil || len(failedEval.FailedTGAllocs) == 0 {
		return
	}

	c.Ui.Output(c.Colorize().Color("\n[bold]Placement Failure[reset]"))

	sorted := sortedTaskGroupFromMetrics(failedEval.FailedTGAllocs)
	for i, tg := range sorted {
		if i >= maxFailedTGs {
			break
		}

		c.Ui.Output(fmt.Sprintf("Task Group %q:", tg))
		metrics := failedEval.FailedTGAllocs[tg]
		c.Ui.Output(formatAllocMetrics(metrics, false, "  "))
		if i != len(sorted)-1 {
			c.Ui.Output("")
		}
	}

	if len(sorted) > maxFailedTGs {
		trunc := fmt.Sprintf("\nPlacement failures truncated. To see remainder run:\nnomad eval-status %s", failedEval.ID)
		c.Ui.Output(trunc)
	}
}

func createJsonJobsOutput(client *api.Client, allAllocs bool, jobs ...NamespacedID) ([]JobJson, error) {
	jsonJobs := make([]JobJson, len(jobs))

	for i, pair := range jobs {
		q := &api.QueryOptions{Namespace: pair.Namespace}

		summary, _, err := client.Jobs().Summary(pair.ID, q)
		if err != nil {
			return nil, fmt.Errorf("Error querying job summary: %s", err)
		}

		allocations, _, err := client.Jobs().Allocations(pair.ID, allAllocs, q)
		if err != nil {
			return nil, fmt.Errorf("Error querying job allocations: %s", err)
		}

		latestDeployment, _, err := client.Jobs().LatestDeployment(pair.ID, q)
		if err != nil {
			return nil, fmt.Errorf("Error querying latest job deployment: %s", err)
		}

		evals, _, err := client.Jobs().Evaluations(pair.ID, q)
		if err != nil {
			return nil, fmt.Errorf("Error querying job evaluations: %s", err)
		}

		jsonJobs[i] = JobJson{
			Summary:          summary,
			Allocations:      allocations,
			LatestDeployment: latestDeployment,
			Evaluations:      evals,
		}
	}

	return jsonJobs, nil
}

// list general information about a list of jobs
func createStatusListOutput(jobs []*api.JobListStub, displayNS bool) string {
	out := make([]string, len(jobs)+1)
	if displayNS {
		out[0] = "ID|Namespace|Type|Priority|Status|Submit Date"
		for i, job := range jobs {
			out[i+1] = fmt.Sprintf("%s|%s|%s|%d|%s|%s",
				job.ID,
				job.JobSummary.Namespace,
				getTypeString(job),
				job.Priority,
				getStatusString(job.Status, &job.Stop),
				formatTime(time.Unix(0, job.SubmitTime)))
		}
	} else {
		out[0] = "ID|Type|Priority|Status|Submit Date"
		for i, job := range jobs {
			out[i+1] = fmt.Sprintf("%s|%s|%d|%s|%s",
				job.ID,
				getTypeString(job),
				job.Priority,
				getStatusString(job.Status, &job.Stop),
				formatTime(time.Unix(0, job.SubmitTime)))
		}
	}
	return formatList(out)
}

func getTypeString(job *api.JobListStub) string {
	t := job.Type

	if job.Periodic {
		t += "/periodic"
	}

	if job.ParameterizedJob {
		t += "/parameterized"
	}

	return t
}

func getStatusString(status string, stop *bool) string {
	if stop != nil && *stop {
		return fmt.Sprintf("%s (stopped)", status)
	}
	return status
}
