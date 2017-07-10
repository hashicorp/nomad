package command

import (
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

  ` + generalOptionsUsage() + `

Inspect Options:

  -json
    Output the job in its JSON format.

  -t
    Format and display job using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *InspectCommand) Synopsis() string {
	return "Inspect a submitted job"
}

func (c *InspectCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet("inspect", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// If args not specified but output format is specified, format and output the jobs data list
	if len(args) == 0 && json || len(tmpl) > 0 {
		jobs, _, err := client.Jobs().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying jobs: %v", err))
			return 1
		}

		out, err := Format(json, tmpl, jobs)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// Check that we got exactly one job
	if len(args) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}
	jobID := args[0]

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
		c.Ui.Output(fmt.Sprintf("Prefix matched multiple jobs\n\n%s", createStatusListOutput(jobs)))
		return 0
	}

	// Prefix lookup matched a single job
	job, _, err := client.Jobs().Info(jobs[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error inspecting job: %s", err))
		return 1
	}

	// If output format is specified, format and output the data
	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, job)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// Print the contents of the job
	req := api.RegisterJobRequest{Job: job}
	f, err := DataFormat("json", "")
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting formatter: %s", err))
		return 1
	}

	out, err := f.TransformData(req)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error formatting the data: %s", err))
		return 1
	}
	c.Ui.Output(out)
	return 0
}
