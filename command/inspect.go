package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
)

type InspectCommand struct {
	Meta
}

func (c *InspectCommand) Help() string {
	helpText := `
Usage: nomad inspect [options] <job>

  Inspect is used to see the specification of a submitted job.

General Options:

  ` + generalOptionsUsage()

	return strings.TrimSpace(helpText)
}

func (c *InspectCommand) Synopsis() string {
	return "Inspect a submitted job"
}

func (c *InspectCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("inspect", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one job
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}
	jobID := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check if the job exists
	jobs, _, err := client.Jobs().PrefixList(jobID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error inspecting job: %s", err))
		return 1
	}
	if len(jobs) == 0 {
		c.Ui.Error(fmt.Sprintf("No job(s) with prefix or id %q found", jobID))
		return 1
	}
	if len(jobs) > 1 && strings.TrimSpace(jobID) != jobs[0].ID {
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
	job, _, err := client.Jobs().Info(jobs[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error inspecting job: %s", err))
		return 1
	}

	// Print the contents of the job
	req := api.RegisterJobRequest{Job: job}
	buf, err := json.MarshalIndent(req, "", "    ")
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error converting job: %s", err))
		return 1
	}

	c.Ui.Output(string(buf))
	return 0
}
