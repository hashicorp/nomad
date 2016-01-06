package command

import (
	"fmt"
	"strings"
)

type StopCommand struct {
	Meta
}

func (c *StopCommand) Help() string {
	helpText := `
Usage: nomad stop [options] <job>

  Stop an existing job. This command is used to signal allocations
  to shut down for the given job ID. Upon successful deregistraion,
  an interactive monitor session will start to display log lines as
  the job unwinds its allocations and completes shutting down. It
  is safe to exit the monitor early using ctrl+c.

General Options:

  ` + generalOptionsUsage() + `

Stop Options:

  -detach
    Return immediately instead of entering monitor mode. After the
    deregister command is submitted, a new evaluation ID is printed
    to the screen, which can be used to call up a monitor later if
    needed using the eval-monitor command.
`
	return strings.TrimSpace(helpText)
}

func (c *StopCommand) Synopsis() string {
	return "Stop a running job"
}

func (c *StopCommand) Run(args []string) int {
	var detach bool

	flags := c.Meta.FlagSet("stop", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")

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
	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		jobs, _, err := client.Jobs().PrefixList(jobID)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error deregistering job: %s", err))
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
			c.Ui.Output(fmt.Sprintf("Please disambiguate the desired job\n\n%s", formatList(out)))
			return 0
		}
		// Prefix lookup matched a single job
		job, _, err = client.Jobs().Info(jobs[0].ID, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error deregistering job: %s", err))
			return 1
		}
	}

	// Invoke the stop
	evalID, _, err := client.Jobs().Deregister(job.ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deregistering job: %s", err))
		return 1
	}

	if detach {
		c.Ui.Output(evalID)
		return 0
	}

	// Start monitoring the stop eval
	mon := newMonitor(c.Ui, client)
	return mon.monitor(evalID)
}
