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

  Display status and diagnostic information about an allocation.

General Options:

  ` + generalOptionsUsage()
	return strings.TrimSpace(helpText)
}

func (c *AllocStatusCommand) Synopsis() string {
	return "Display allocation status information and metrics"
}

func (c *AllocStatusCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("alloc-status", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no jobs or exactly one.
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}
	allocID := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Query the allocation
	alloc, _, err := client.Allocations().Info(allocID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	// Format the allocation
	basic := []string{
		fmt.Sprintf("ID|%s", alloc.ID),
		fmt.Sprintf("EvalID|%s", alloc.EvalID),
		fmt.Sprintf("Name|%s", alloc.Name),
		fmt.Sprintf("NodeID|%s", alloc.NodeID),
		fmt.Sprintf("JobID|%s", alloc.JobID),
		fmt.Sprintf("TaskGroup|%s", alloc.TaskGroup),
		fmt.Sprintf("DesiredStatus|%s", alloc.DesiredStatus),
		fmt.Sprintf("DesiredDescription|%s", alloc.DesiredDescription),
		fmt.Sprintf("ClientStatus|%s", alloc.ClientStatus),
		fmt.Sprintf("ClientDescription|%s", alloc.ClientDescription),
		fmt.Sprintf("NodesEvaluated|%d", alloc.Metrics.NodesEvaluated),
		fmt.Sprintf("NodesFiltered|%d", alloc.Metrics.NodesFiltered),
	}

	// Format exhaustion info
	var exInfo []string
	if ne := alloc.Metrics.NodesExhausted; ne > 0 {
		exInfo = append(exInfo, fmt.Sprintf("Node resources (exhausted %d)", ne))
	}
	for class, num := range alloc.Metrics.ClassExhausted {
		exInfo = append(exInfo, fmt.Sprintf("Class %q (exhausted %d)", class, num))
	}
	for dim, num := range alloc.Metrics.DimensionExhausted {
		exInfo = append(exInfo, fmt.Sprintf("Dimension %q (exhausted %d)", dim, num))
	}

	// Format the filter info
	var filterInfo []string
	for class, num := range alloc.Metrics.ClassFiltered {
		filterInfo = append(filterInfo, fmt.Sprintf("Class %q (filtered %d)", class, num))
	}
	for cs, num := range alloc.Metrics.ConstraintFiltered {
		filterInfo = append(filterInfo, fmt.Sprintf("Constraint %q (filtered %d)", cs, num))
	}

	// Dump the output
	c.Ui.Output(formatKV(basic))
	if len(exInfo) > 0 {
		c.Ui.Output("\n==> Nodes Exhausted")
		c.Ui.Output(strings.Join(exInfo, "\n"))
	}
	if len(filterInfo) > 0 {
		c.Ui.Output("\n==> Filters Applied")
		c.Ui.Output(strings.Join(filterInfo, "\n"))
	}
	return 0
}
