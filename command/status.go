package command

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// maxFailedTGs is the maximum number of task groups we show failure reasons
	// for before defering to eval-status
	maxFailedTGs = 5
)

type StatusCommand struct {
	Meta
	length    int
	evals     bool
	allAllocs bool
	verbose   bool
}

func (c *StatusCommand) Help() string {
	helpText := `
Usage: nomad status [options] <job>

  Display status information about jobs. If no job ID is given,
  a list of all known jobs will be dumped.

General Options:

  ` + generalOptionsUsage() + `

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

func (c *StatusCommand) Synopsis() string {
	return "Display status information about jobs"
}

func (c *StatusCommand) Run(args []string) int {
	var short bool

	flags := c.Meta.FlagSet("status", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&short, "short", false, "")
	flags.BoolVar(&c.evals, "evals", false, "")
	flags.BoolVar(&c.allAllocs, "all-allocs", false, "")
	flags.BoolVar(&c.verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no jobs or exactly one.
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error(c.Help())
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

	// Invoke list mode if no job ID.
	if len(args) == 0 {
		jobs, _, err := client.Jobs().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying jobs: %s", err))
			return 1
		}

		if len(jobs) == 0 {
			// No output if we have no jobs
			c.Ui.Output("No running jobs")
		} else {
			c.Ui.Output(createStatusListOutput(jobs))
		}
		return 0
	}

	// Try querying the job
	jobID := args[0]
	jobs, _, err := client.Jobs().PrefixList(jobID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying job: %s", err))
		return 1
	}
	if len(jobs) == 0 {
		c.Ui.Error(fmt.Sprintf("No job(s) with prefix or id %q found", jobID))
		return 1
	}
	if len(jobs) > 1 && strings.TrimSpace(jobID) != jobs[0].ID {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple jobs\n\n%s", createStatusListOutput(jobs)))
		return 1
	}
	// Prefix lookup matched a single job
	job, _, err := client.Jobs().Info(jobs[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying job: %s", err))
		return 1
	}

	periodic := job.IsPeriodic()
	parameterized := job.IsParameterized()

	// Format the job info
	basic := []string{
		fmt.Sprintf("ID|%s", *job.ID),
		fmt.Sprintf("Name|%s", *job.Name),
		fmt.Sprintf("Submit Date|%s", formatTime(time.Unix(0, *job.SubmitTime))),
		fmt.Sprintf("Type|%s", *job.Type),
		fmt.Sprintf("Priority|%d", *job.Priority),
		fmt.Sprintf("Datacenters|%s", strings.Join(job.Datacenters, ",")),
		fmt.Sprintf("Status|%s", getStatusString(*job.Status, *job.Stop)),
		fmt.Sprintf("Periodic|%v", periodic),
		fmt.Sprintf("Parameterized|%v", parameterized),
	}

	if periodic && !parameterized {
		if *job.Stop {
			basic = append(basic, fmt.Sprintf("Next Periodic Launch|none (job stopped)"))
		} else {
			location, err := job.Periodic.GetLocation()
			if err == nil {
				now := time.Now().In(location)
				next := job.Periodic.Next(now)
				basic = append(basic, fmt.Sprintf("Next Periodic Launch|%s",
					fmt.Sprintf("%s (%s from now)",
						formatTime(next), formatTimeDifference(now, next, time.Second))))
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
func (c *StatusCommand) outputPeriodicInfo(client *api.Client, job *api.Job) error {
	// Output the summary
	if err := c.outputJobSummary(client, job); err != nil {
		return err
	}

	// Generate the prefix that matches launched jobs from the periodic job.
	prefix := fmt.Sprintf("%s%s", *job.ID, structs.PeriodicLaunchSuffix)
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
func (c *StatusCommand) outputParameterizedInfo(client *api.Client, job *api.Job) error {
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
	prefix := fmt.Sprintf("%s%s", *job.ID, structs.DispatchLaunchSuffix)
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
func (c *StatusCommand) outputJobInfo(client *api.Client, job *api.Job) error {

	// Query the allocations
	jobAllocs, _, err := client.Jobs().Allocations(*job.ID, c.allAllocs, nil)
	if err != nil {
		return fmt.Errorf("Error querying job allocations: %s", err)
	}

	// Query the evaluations
	jobEvals, _, err := client.Jobs().Evaluations(*job.ID, nil)
	if err != nil {
		return fmt.Errorf("Error querying job evaluations: %s", err)
	}

	latestDeployment, _, err := client.Jobs().LatestDeployment(*job.ID, nil)
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

	if latestDeployment != nil {
		c.Ui.Output(c.Colorize().Color("\n[bold]Latest Deployment[reset]"))
		c.Ui.Output(c.Colorize().Color(c.formatDeployment(latestDeployment)))
	}

	// Format the allocs
	c.Ui.Output(c.Colorize().Color("\n[bold]Allocations[reset]"))
	c.Ui.Output(formatAllocListStubs(jobAllocs, c.verbose, c.length))
	return nil
}

func (c *StatusCommand) formatDeployment(d *api.Deployment) string {
	// Format the high-level elements
	high := []string{
		fmt.Sprintf("ID|%s", limit(d.ID, c.length)),
		fmt.Sprintf("Status|%s", d.Status),
		fmt.Sprintf("Description|%s", d.StatusDescription),
	}

	base := formatKV(high)
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
		allocs[0] = "ID|Eval ID|Node ID|Task Group|Version|Desired|Status|Created At"
		for i, alloc := range stubs {
			allocs[i+1] = fmt.Sprintf("%s|%s|%s|%s|%d|%s|%s|%s",
				limit(alloc.ID, uuidLength),
				limit(alloc.EvalID, uuidLength),
				limit(alloc.NodeID, uuidLength),
				alloc.TaskGroup,
				alloc.JobVersion,
				alloc.DesiredStatus,
				alloc.ClientStatus,
				formatUnixNanoTime(alloc.CreateTime))
		}
	} else {
		allocs[0] = "ID|Node ID|Task Group|Version|Desired|Status|Created At"
		for i, alloc := range stubs {
			allocs[i+1] = fmt.Sprintf("%s|%s|%s|%d|%s|%s|%s",
				limit(alloc.ID, uuidLength),
				limit(alloc.NodeID, uuidLength),
				alloc.TaskGroup,
				alloc.JobVersion,
				alloc.DesiredStatus,
				alloc.ClientStatus,
				formatUnixNanoTime(alloc.CreateTime))
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
		allocs[0] = "ID|Eval ID|Node ID|Task Group|Version|Desired|Status|Created At"
		for i, alloc := range allocations {
			allocs[i+1] = fmt.Sprintf("%s|%s|%s|%s|%d|%s|%s|%s",
				limit(alloc.ID, uuidLength),
				limit(alloc.EvalID, uuidLength),
				limit(alloc.NodeID, uuidLength),
				alloc.TaskGroup,
				*alloc.Job.Version,
				alloc.DesiredStatus,
				alloc.ClientStatus,
				formatUnixNanoTime(alloc.CreateTime))
		}
	} else {
		allocs[0] = "ID|Node ID|Task Group|Version|Desired|Status|Created At"
		for i, alloc := range allocations {
			allocs[i+1] = fmt.Sprintf("%s|%s|%s|%d|%s|%s|%s",
				limit(alloc.ID, uuidLength),
				limit(alloc.NodeID, uuidLength),
				alloc.TaskGroup,
				*alloc.Job.Version,
				alloc.DesiredStatus,
				alloc.ClientStatus,
				formatUnixNanoTime(alloc.CreateTime))
		}
	}

	return formatList(allocs)
}

// outputJobSummary displays the given jobs summary and children job summary
// where appropriate
func (c *StatusCommand) outputJobSummary(client *api.Client, job *api.Job) error {
	// Query the summary
	summary, _, err := client.Jobs().Summary(*job.ID, nil)
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
		summaries[0] = "Task Group|Queued|Starting|Running|Failed|Complete|Lost"
		taskGroups := make([]string, 0, len(summary.Summary))
		for taskGroup := range summary.Summary {
			taskGroups = append(taskGroups, taskGroup)
		}
		sort.Strings(taskGroups)
		for idx, taskGroup := range taskGroups {
			tgs := summary.Summary[taskGroup]
			summaries[idx+1] = fmt.Sprintf("%s|%d|%d|%d|%d|%d|%d",
				taskGroup, tgs.Queued, tgs.Starting,
				tgs.Running, tgs.Failed,
				tgs.Complete, tgs.Lost,
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

func (c *StatusCommand) outputFailedPlacements(failedEval *api.Evaluation) {
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

// list general information about a list of jobs
func createStatusListOutput(jobs []*api.JobListStub) string {
	out := make([]string, len(jobs)+1)
	out[0] = "ID|Type|Priority|Status|Submit Date"
	for i, job := range jobs {
		out[i+1] = fmt.Sprintf("%s|%s|%d|%s|%s",
			job.ID,
			getTypeString(job),
			job.Priority,
			getStatusString(job.Status, job.Stop),
			formatTime(time.Unix(0, job.SubmitTime)))
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

func getStatusString(status string, stop bool) string {
	if stop {
		return fmt.Sprintf("%s (stopped)", status)
	}
	return status
}
