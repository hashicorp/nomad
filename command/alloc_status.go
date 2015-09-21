package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
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

	// Dump any allocation data
	dumpAllocStatus(c.Ui, alloc)
	return 0
}

// dumpAllocStatus is a helper to generate a more user-friendly error message
// for scheduling failures, displaying a high level status of why the job
// could not be scheduled out.
func dumpAllocStatus(ui cli.Ui, alloc *api.Allocation) {
	// Print filter stats
	ui.Output(fmt.Sprintf("Allocation %q status %q (%d/%d nodes filtered)",
		alloc.ID, alloc.ClientStatus,
		alloc.Metrics.NodesFiltered, alloc.Metrics.NodesEvaluated))

	// Print exhaustion info
	if ne := alloc.Metrics.NodesExhausted; ne > 0 {
		ui.Output(fmt.Sprintf("  * Resources exhausted on %d nodes", ne))
	}
	for class, num := range alloc.Metrics.ClassExhausted {
		ui.Output(fmt.Sprintf("  * Class %q exhausted on %d nodes", class, num))
	}
	for dim, num := range alloc.Metrics.DimensionExhausted {
		ui.Output(fmt.Sprintf("  * Dimension %q exhausted on %d nodes", dim, num))
	}

	// Print filter info
	for class, num := range alloc.Metrics.ClassFiltered {
		ui.Output(fmt.Sprintf("  * Class %q filtered %d nodes", class, num))
	}
	for cs, num := range alloc.Metrics.ConstraintFiltered {
		ui.Output(fmt.Sprintf("  * Constraint %q filtered %d nodes", cs, num))
	}
}
