package command

import (
	"fmt"
	"strings"
)

type EvalMonitorCommand struct {
	Meta
}

func (c *EvalMonitorCommand) Help() string {
	helpText := `
Usage: nomad eval-monitor [options] <evaluation>

  Start an interactive monitoring session for an existing evaluation.
  The monitor command periodically polls for information about the
  provided evaluation, including status updates, new allocations,
  updates to allocations, and failures. Status is printed in near
  real-time to the terminal.

  The command will exit when the given evaluation reaches a terminal
  state (completed or failed). The exit code indicates the status
  of the monitoring only; it does not reflect the evaluation status.

General Options:

  ` + generalOptionsUsage()
	return strings.TrimSpace(helpText)
}

func (c *EvalMonitorCommand) Synopsis() string {
	return "Monitor an evaluation interactively"
}

func (c *EvalMonitorCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("eval-monitor", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one eval ID
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}
	evalID := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Start monitoring
	mon := newMonitor(c.Ui, client)
	return mon.monitor(evalID)
}
