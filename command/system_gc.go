package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type SystemGCCommand struct {
	Meta
}

func (c *SystemGCCommand) Help() string {
	helpText := `
Usage: nomad system gc [options]

  Initializes a garbage collection of jobs, evaluations, allocations, and nodes.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)
	return strings.TrimSpace(helpText)
}

func (c *SystemGCCommand) Synopsis() string {
	return "Run the system garbage collection process"
}

func (c *SystemGCCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *SystemGCCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *SystemGCCommand) Name() string { return "system gc" }

func (c *SystemGCCommand) Run(args []string) int {

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if err := client.System().GarbageCollect(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error running system garbage-collection: %s", err))
		return 1
	}
	return 0
}
