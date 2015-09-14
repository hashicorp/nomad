package command

import (
	"fmt"
	"strings"

	"github.com/ryanuber/columnize"
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

  ` + generalOptionsUsage()
	return strings.TrimSpace(helpText)
}

func (c *StatusCommand) Synopsis() string {
	return "Display status information about jobs"
}

func (c *StatusCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("status", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
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
		c.Ui.Output(columnize.SimpleFormat(out))
		return 0
	}

	// Try querying the job
	jobID := args[0]
	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying job: %s", err))
		return 1
	}

	// Format the job info
	basic := []string{
		fmt.Sprintf("ID | %s", job.ID),
		fmt.Sprintf("Name | %s", job.Name),
		fmt.Sprintf("Type | %s", job.Type),
		fmt.Sprintf("Priority | %d", job.Priority),
		fmt.Sprintf("Datacenters | %s", strings.Join(job.Datacenters, ",")),
		fmt.Sprintf("Status | %s", job.Status),
		fmt.Sprintf("StatusDescription | %s", job.StatusDescription),
	}
	c.Ui.Output(columnize.SimpleFormat(basic))

	return 0
}
