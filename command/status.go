package command

import (
	"fmt"
	"strings"
)

type StatusCommand struct {
	Meta
}

func (c *StatusCommand) Help() string {
	helpText := `
Usage: nomad status [options] [job]

  Display status information about jobs. If no job ID is given,
  a list of all known jobs will be dumped.

General Options:

  ` + generalOptionsUsage() + `

Status Options:

  -short
    Display short output. Used only when a single job is being
    queried, and drops verbose information about allocations
    and evaluations.
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

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no jobs or exactly one.
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	// Invoke list mode if no job ID.
	if len(args) == 0 {
		jobs, _, err := client.Jobs().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying jobs: %v", err))
			return 1
		}

		// No output if we have no jobs
		if len(jobs) == 0 {
			return 0
		}

		out := make([]string, len(jobs)+1)
		out[0] = "ID|Type|Priority|Status"
		for i, job := range jobs {
			out[i+1] = fmt.Sprintf("%s|%s|%d|%s",
				job.ID,
				job.Type,
				job.Priority,
				job.Status)
		}
		c.Ui.Output(formatList(out))
		return 0
	}

	// Try querying the job
	jobID := args[0]
	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying job: %v", err))
		return 1
	}

	// Format the job info
	basic := []string{
		fmt.Sprintf("ID|%s", job.ID),
		fmt.Sprintf("Name|%s", job.Name),
		fmt.Sprintf("Type|%s", job.Type),
		fmt.Sprintf("Priority|%d", job.Priority),
		fmt.Sprintf("Datacenters|%s", strings.Join(job.Datacenters, ",")),
		fmt.Sprintf("Status|%s", job.Status),
	}

	var evals, allocs []string
	if !short {
		// Query the evaluations
		jobEvals, _, err := client.Jobs().Evaluations(jobID, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying job evaluations: %v", err))
			return 1
		}

		// Query the allocations
		jobAllocs, _, err := client.Jobs().Allocations(jobID, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying job allocations: %v", err))
			return 1
		}

		// Format the evals
		evals = make([]string, len(jobEvals)+1)
		evals[0] = "ID|Priority|TriggeredBy|Status"
		for i, eval := range jobEvals {
			evals[i+1] = fmt.Sprintf("%s|%d|%s|%s",
				eval.ID,
				eval.Priority,
				eval.TriggeredBy,
				eval.Status)
		}

		// Format the allocs
		allocs = make([]string, len(jobAllocs)+1)
		allocs[0] = "ID|EvalID|NodeID|TaskGroup|Desired|Status"
		for i, alloc := range jobAllocs {
			allocs[i+1] = fmt.Sprintf("%s|%s|%s|%s|%s|%s",
				alloc.ID,
				alloc.EvalID,
				alloc.NodeID,
				alloc.TaskGroup,
				alloc.DesiredStatus,
				alloc.ClientStatus)
		}
	}

	// Dump the output
	c.Ui.Output(formatKV(basic))
	if !short {
		c.Ui.Output("\n==> Evaluations")
		c.Ui.Output(formatList(evals))
		c.Ui.Output("\n==> Allocations")
		c.Ui.Output(formatList(allocs))
	}
	return 0
}
