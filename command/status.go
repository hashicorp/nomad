package command

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
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
		jobs, _, err := client.Jobs().PrefixList(jobID)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying job: %s", err))
			return 1
		}
		if len(jobs) == 0 {
			c.Ui.Error(fmt.Sprintf("No job(s) with prefix or id %q found", jobID))
			return 1
		}
		if len(jobs) > 1 {
			out := make([]string, len(jobs)+1)
			out[0] = "ID|Type|Priority|Status"
			for i, job := range jobs {
				out[i+1] = fmt.Sprintf("%s|%s|%d|%s",
					job.ID,
					job.Type,
					job.Priority,
					job.Status)
			}
			c.Ui.Output(fmt.Sprintf("Prefix matched multiple jobs\n\n%s", formatList(out)))
			return 0
		}
		// Prefix lookup matched a single job
		job, _, err = client.Jobs().Info(jobs[0].ID, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying job: %s", err))
			return 1
		}
	}

	// Check if it is periodic
	sJob, err := convertApiJob(job)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error converting job: %s", err))
		return 1
	}
	periodic := sJob.IsPeriodic()

	// Format the job info
	basic := []string{
		fmt.Sprintf("ID|%s", job.ID),
		fmt.Sprintf("Name|%s", job.Name),
		fmt.Sprintf("Type|%s", job.Type),
		fmt.Sprintf("Priority|%d", job.Priority),
		fmt.Sprintf("Datacenters|%s", strings.Join(job.Datacenters, ",")),
		fmt.Sprintf("Status|%s", job.Status),
		fmt.Sprintf("Periodic|%v", periodic),
	}

	if periodic {
		basic = append(basic, fmt.Sprintf("Next Periodic Launch|%v",
			sJob.Periodic.Next(time.Now())))
	}

	c.Ui.Output(formatKV(basic))

	// Exit early
	if short {
		return 0
	}

	// Print periodic job information
	if periodic {
		if err := c.outputPeriodicInfo(client, job); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		return 0
	}

	if err := c.outputJobInfo(client, job); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	return 0
}

func (c *StatusCommand) outputPeriodicInfo(client *api.Client, job *api.Job) error {
	// Generate the prefix that matches launched jobs from the periodic job.
	prefix := fmt.Sprintf("%s%s", job.ID, structs.PeriodicLaunchSuffix)
	jobs, _, err := client.Jobs().PrefixList(prefix)
	if err != nil {
		return fmt.Errorf("Error querying job: %s", err)
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

	c.Ui.Output(fmt.Sprintf("\nPreviously launched jobs\n%s", formatList(out)))
	return nil
}

func (c *StatusCommand) outputJobInfo(client *api.Client, job *api.Job) error {
	var evals, allocs []string

	// Query the evaluations
	jobEvals, _, err := client.Jobs().Evaluations(job.ID, nil)
	if err != nil {
		return fmt.Errorf("Error querying job evaluations: %s", err)
	}

	// Query the allocations
	jobAllocs, _, err := client.Jobs().Allocations(job.ID, nil)
	if err != nil {
		return fmt.Errorf("Error querying job allocations: %s", err)
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

	c.Ui.Output("\n==> Evaluations")
	c.Ui.Output(formatList(evals))
	c.Ui.Output("\n==> Allocations")
	c.Ui.Output(formatList(allocs))
	return nil
}

// convertApiJob is used to take a *api.Job and convert it to an *struct.Job.
// This function is just a hammer and probably needs to be revisited.
func convertApiJob(in *api.Job) (*structs.Job, error) {
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
	var structJob *structs.Job
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(in); err != nil {
		return nil, err
	}
	if err := gob.NewDecoder(buf).Decode(&structJob); err != nil {
		return nil, err
	}
	return structJob, nil
}
