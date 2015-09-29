package command

import (
	"fmt"
	"strings"
)

type AllocStatusCommand struct {
	Meta
}

func (c *AllocStatusCommand) Help() string {
	helpText := `
Usage: nomad alloc-status [options] <allocation>

  Display information about existing allocations. This command can
  be used to inspect the current status of all allocation,
  including its running status, metadata, and verbose failure
  messages reported by internal subsystems.

General Options:

  ` + generalOptionsUsage()
	return strings.TrimSpace(helpText)
}

func (c *AllocStatusCommand) Synopsis() string {
	return "Display allocation status information and metadata"
}

func (c *AllocStatusCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("alloc-status", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one allocation ID
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}
	allocID := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	// Query the allocation info
	alloc, _, err := client.Allocations().Info(allocID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
		return 1
	}

	// Format the allocation data
	basic := []string{
		fmt.Sprintf("ID|%s", alloc.ID),
		fmt.Sprintf("EvalID|%s", alloc.EvalID),
		fmt.Sprintf("Name|%s", alloc.Name),
		fmt.Sprintf("NodeID|%s", alloc.NodeID),
		fmt.Sprintf("JobID|%s", alloc.JobID),
		fmt.Sprintf("ClientStatus|%s", alloc.ClientStatus),
		fmt.Sprintf("ClientDescription|%s", alloc.ClientDescription),
		fmt.Sprintf("NodesEvaluated|%d", alloc.Metrics.NodesEvaluated),
		fmt.Sprintf("NodesFiltered|%d", alloc.Metrics.NodesFiltered),
		fmt.Sprintf("NodesExhausted|%d", alloc.Metrics.NodesExhausted),
		fmt.Sprintf("AllocationTime|%s", alloc.Metrics.AllocationTime),
		fmt.Sprintf("CoalescedFailures|%d", alloc.Metrics.CoalescedFailures),
	}
	c.Ui.Output(formatKV(basic))

	// Format the detailed status
	c.Ui.Output("\n==> Status")
	dumpAllocStatus(c.Ui, alloc)

	return 0
}
