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
  to shut down for the given job ID. The shutdown happens
  asynchronously, unless the -monitor flag is given, in which case
  an interactive monitor session will display log lines as the
  job unwinds and completes shutting down.

General Options:

  ` + generalOptionsUsage() + `

Stop Options:

  -monitor
    Starts an interactive monitor for the job deregistration. This
    will display logs in the terminal related to the job shutdown,
    and return once the job deregistration has completed.
`
	return strings.TrimSpace(helpText)
}

func (c *StopCommand) Synopsis() string {
	return "Stop a running job"
}

func (c *StopCommand) Run(args []string) int {
	var monitor bool

	flags := c.Meta.FlagSet("stop", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&monitor, "monitor", false, "")

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
	if _, _, err := client.Jobs().Info(jobID, nil); err != nil {
		c.Ui.Error(fmt.Sprintf("Error deregistering job: %s", err))
		return 1
	}

	// Invoke the stop
	evalID, _, err := client.Jobs().Deregister(jobID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deregistering job: %s", err))
		return 1
	}

	if monitor {
		mon := newMonitor(c.Ui, client)
		return mon.monitor(evalID)
	}

	return 0
}
