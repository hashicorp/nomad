package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"
)

type StatusCommand struct {
	Ui cli.Ui
}

func (c *StatusCommand) Help() string {
	helpText := `
Usage: nomad status [options] <job>

  Displays information about the given job. If no job ID
  is given, this command will dump a list of all jobs.

Options:

  -help
    Display this message

  -http-addr
    Address of the Nomad API to connect.
    Default = http://127.0.0.1:4646
`
	return strings.TrimSpace(helpText)
}

func (c *StatusCommand) Synopsis() string {
	return "Displays information about jobs"
}

func (c *StatusCommand) Run(args []string) int {
	var httpAddr *string

	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	httpAddr = HttpAddrFlag(flags)

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no jobs or exactly one.
	if len(flags.Args()) > 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the HTTP client
	client, err := HttpClient(*httpAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed initializing Nomad client: %s", err))
		return 1
	}

	// Invoke list mode if no job ID.
	if len(flags.Args()) == 0 {
		jobs, _, err := client.Jobs().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed querying jobs: %s", err))
			return 1
		}

		out := []string{"ID|Type|Priority|AllAtOnce|Status"}
		for _, job := range jobs {
			out = append(out, fmt.Sprintf("%s|%s|%d|%v|%s",
				job.ID,
				job.Type,
				job.Priority,
				job.AllAtOnce,
				job.Status))
		}
		c.Ui.Output(columnize.SimpleFormat(out))
		return 0
	}

	// Try querying the job
	jobID := flags.Args()[0]
	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed querying job: %s", err))
		return 1
	}

	// Format the job info
	basic := []string{
		fmt.Sprintf("ID | %s", job.ID),
		fmt.Sprintf("Name | %s", job.Name),
		fmt.Sprintf("Type | %s", job.Type),
		fmt.Sprintf("Priority | %d", job.Priority),
		fmt.Sprintf("AllAtOnce | %v", job.AllAtOnce),
		fmt.Sprintf("Datacenters | %s", strings.Join(job.Datacenters, ",")),
		fmt.Sprintf("Status | %s", job.Status),
		fmt.Sprintf("StatusDescription | %s", job.StatusDescription),
	}
	c.Ui.Output(columnize.SimpleFormat(basic))

	return 0
}
